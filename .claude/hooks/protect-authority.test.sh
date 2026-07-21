#!/usr/bin/env bash
# Regression tests for protect-authority.sh. Run: bash .claude/hooks/protect-authority.test.sh
#
# Every case below was found by review, not invented — the BLOCK cases are bypasses the
# first implementation allowed, and the ALLOW cases are false positives it produced.
# A false positive is the more dangerous failure: it gets the hook disabled.
#
# Payloads live in this file as JSON on stdin; nothing touches the repo.
set -uo pipefail
HOOK="$(dirname "${BASH_SOURCE[0]}")/protect-authority.sh"
DEC='docs/product/18-philosophy/decisions.md'
SRC='docs/product/00-sources'
pass=0; fail=0

# $1 expect (block|allow) · $2 label · $3 JSON payload
t() {
  local rc; rc=0
  printf '%s' "$3" | bash "$HOOK" >/dev/null 2>&1 || rc=$?
  # rc must be exactly 0 (allow) or 2 (block). Anything else is a crash — treating a crash
  # as "allow" would score a broken hook as passing every allow case.
  local got
  case "$rc" in 0) got=allow ;; 2) got=block ;; *) got="crash(rc=$rc)" ;; esac
  if [ "$got" = "$1" ]; then pass=$((pass+1));
  else fail=$((fail+1)); printf '  FAIL  expected %-5s got %-9s : %s\n' "$1" "$got" "$2"; fi
}
bash_cmd() { printf '{"tool_input":{"command":%s}}' "$(printf '%s' "$1" | jq -Rs .)"; }
file_tool() { printf '{"tool_input":{"file_path":"%s"}}' "$1"; }

echo "== shape 1: structured file-path tools (must not regress) =="
t block "Write to decisions.md"        "$(file_tool "/repo/$DEC")"
t block "Write into 00-sources"        "$(file_tool "/repo/$SRC/GOV-002.md")"
t allow "Write elsewhere"              "$(file_tool "/repo/tasks/eops/O6f.md")"

echo "== shape 2: redirects =="
t block "append >>"                    "$(bash_cmd "echo ratified >> $DEC")"
t block "overwrite >"                  "$(bash_cmd "echo x > $SRC/GOV-002.md")"
t block "clobber >|"                   "$(bash_cmd "echo x >| $DEC")"
t block "quoted target"                "$(bash_cmd "echo x > \"$DEC\"")"
t block "fd-qualified 1>"              "$(bash_cmd "echo x 1> $DEC")"
t allow "read-out redirect"            "$(bash_cmd "cat $DEC > /tmp/copy.md")"

echo "== shape 2: in-place editors =="
t block "sed -i"                       "$(bash_cmd "sed -i s/a/b/ $DEC")"
t block "perl -pi -e"                  "$(bash_cmd "perl -pi -e 's/a/b/' $DEC")"

echo "== shape 2: destructive verbs =="
t block "rm file"                      "$(bash_cmd "rm $DEC")"
t block "rm -rf dir (no slash)"        "$(bash_cmd "rm -rf $SRC")"
t block "rm -rf dir/ (slash)"          "$(bash_cmd "rm -rf $SRC/")"
t block "truncate"                     "$(bash_cmd "truncate -s 0 $DEC")"

echo "== shape 2: copy/move destination vs source =="
t block "mv onto protected"            "$(bash_cmd "mv /tmp/x $DEC")"
t block "cp onto protected"            "$(bash_cmd "cp /tmp/stamped.md $DEC")"
t block "tee onto protected"           "$(bash_cmd "echo x | tee $DEC")"
t allow "cp FROM protected (backup)"   "$(bash_cmd "cp $DEC /tmp/backup.md")"
t allow "cp -r FROM protected"         "$(bash_cmd "cp -r $SRC/ /tmp/archive/")"

echo "== shape 2: git-mediated writes =="
t block "git checkout -- path"         "$(bash_cmd "git checkout HEAD~5 -- $DEC")"
t block "git restore --source"         "$(bash_cmd "git restore --source=HEAD~3 $DEC")"
t allow "git log (read)"               "$(bash_cmd "git log --oneline $DEC")"
t allow "git diff --stat (read)"       "$(bash_cmd "git diff --stat $DEC")"

echo "== shape 2: interpreter writes =="
t block "python open(...,w)"           "$(bash_cmd "python3 -c \"open('$DEC','w')\"")"
t block "python write_text"            "$(bash_cmd "python3 -c \"pathlib.Path('$DEC').write_text('x')\"")"
t block "node appendFileSync"          "$(bash_cmd "node -e \"require('fs').appendFileSync('$DEC','x')\"")"

echo "== shape 2: cd + relative =="
t block "cd then relative append"      "$(bash_cmd "cd docs/product/18-philosophy && echo x >> decisions.md")"

echo "== FALSE POSITIVES — these get the hook disabled if they fail =="
t allow "cat"                          "$(bash_cmd "cat $DEC")"
t allow "grep"                         "$(bash_cmd "grep -n ADR $DEC")"
t allow "grep for 'rm' in content"     "$(bash_cmd "grep -n 'rm' $DEC")"
t allow "ripgrep openapi in 00-sources" "$(bash_cmd "rg openapi $SRC/")"
t allow "awk matching /open/"          "$(bash_cmd "awk '/open/' $DEC")"
t allow "head/wc"                      "$(bash_cmd "wc -l $DEC")"
t allow "ordinary build"               "$(bash_cmd "go build ./... && go vet ./...")"
t allow "write elsewhere"              "$(bash_cmd "echo x > /tmp/scratch.md")"
t allow "write a task file"            "$(bash_cmd "echo x > tasks/eops/O6f.md")"
t allow "heredoc documenting the hook" "$(bash_cmd "cat > /tmp/note.md <<'EOF'
repro: echo x >> $DEC
EOF")"

echo "== round-2 bypasses (all were ALLOWED by the first version) =="
t block "cp dest + trailing 2>/dev/null" "$(bash_cmd "cp /tmp/stamped.md $DEC 2>/dev/null")"
t block "mv dest + trailing redirect"   "$(bash_cmd "mv /tmp/x $DEC 2>/dev/null")"
t block "tee dest + >/dev/null"         "$(bash_cmd "echo x | tee $DEC >/dev/null")"
t block "cp dest + trailing flag"       "$(bash_cmd "cp /tmp/x $DEC -v")"
t block "interpreter heredoc (bash)"    "$(bash_cmd "bash <<'EOF'
echo ratified >> $DEC
EOF")"
t block "interpreter heredoc (sh rm)"   "$(bash_cmd "sh <<'SH'
rm -rf $SRC
SH")"
t block "interpreter heredoc (python)"  "$(bash_cmd "python3 <<'PY'
open('$DEC','w').write('x')
PY")"
t block "patch"                         "$(bash_cmd "patch $DEC < /tmp/p.diff")"
t block "touch new authority doc"       "$(bash_cmd "touch $SRC/GOV-999.md")"
t block "git checkout . (pathless)"     "$(bash_cmd "git checkout .")"
t block "git reset --hard"              "$(bash_cmd "git reset --hard HEAD~1")"
t block "git stash pop"                 "$(bash_cmd "git stash pop")"
t block "git clean -fd on 00-sources"   "$(bash_cmd "git clean -fd $SRC")"

echo "== round-2 false positives (all were BLOCKED by the first version) =="
t allow "multi-line heredoc finding"    "$(bash_cmd "cat > /tmp/finding.md <<'EOF'
Repro
the hook missed this:
echo x >> $DEC
EOF")"
t allow "read authority && write elsewhere" "$(bash_cmd "python3 -c \"print(open('$DEC').read())\" && node -e \"require('fs').writeFileSync('/tmp/o','x')\"")"
t allow "read authority && echo 'w'"    "$(bash_cmd "python3 -c \"print(open('$DEC').read())\" && echo 'w'")"

echo "== founder escape hatch =="
t allow "STELOIT_RATIFY=1 cp"          "$(bash_cmd "STELOIT_RATIFY=1 cp /tmp/stamped.md $DEC")"

echo "== degenerate input =="
t allow "no command, no path"          '{"tool_input":{}}'
t allow "empty tool_input"             '{}'

echo
if [ "$fail" -eq 0 ]; then echo "OK: $pass passed, 0 failed"; exit 0; fi
echo "FAILED: $pass passed, $fail failed"; exit 1
