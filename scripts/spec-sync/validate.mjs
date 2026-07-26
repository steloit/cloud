#!/usr/bin/env node
// Validate repo invariants: task files (schema, deps, context packs, caps), the CLAUDE.md
// entrypoint symlink, and the .claude/agents/ directory. Exit 1 on any failure. No dependencies.
import { readFileSync, existsSync, readdirSync, lstatSync, readlinkSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { loadTasks, parseFrontmatter } from "./lib.mjs";

// fileURLToPath, not .pathname: the latter percent-encodes, so a checkout under a path
// containing a space resolves to a directory that does not exist.
const repoPath = (rel) => fileURLToPath(new URL(rel, import.meta.url));
const CONTEXTS_DIR = repoPath("../../contexts/");
const AGENTS_MD = repoPath("../../AGENTS.md");
const CLAUDE_MD = repoPath("../../CLAUDE.md");
const AGENTS_DIR = repoPath("../../.claude/agents/");
const AGENTS_README = `${AGENTS_DIR}README.md`;
// ADR-0008 names these two as the mandatory pipeline reviewers. Compared case-insensitively
// because harness name resolution is case-insensitive (verified: `Reviewer` folds to `reviewer`),
// so a file named QA.md would otherwise escape the check while still serving `subagent_type: qa`.
const REVIEW_AGENTS = ["reviewer", "qa"];
// These are the tools the two reviewers may REQUEST. Bash is permitted because they need
// `git diff`/`go test`/`grep` — it is NOT read-only, and this list must never be cited as
// proof that a reviewer cannot write. See .claude/agents/README.md and O6d.
const READONLY_TOOLS = new Set(["Read", "Grep", "Glob", "Bash"]);

const schema = JSON.parse(readFileSync(new URL("./task.schema.json", import.meta.url), "utf8"));
const errors = [];
const err = (f, msg) => errors.push(`${f.replace(/^.*\/tasks\//, "tasks/")}: ${msg}`);

const tasks = loadTasks(true);
const workItems = tasks.filter((t) => !t.file.endsWith("_epic.md"));
const ids = new Set(tasks.map((t) => t.meta.id)); // epics are valid dep targets too
const epicIds = new Set(tasks.filter((t) => t.file.endsWith("_epic.md")).map((t) => t.meta.id));
const packs = existsSync(CONTEXTS_DIR)
  ? new Set(readdirSync(CONTEXTS_DIR).filter((f) => f.endsWith(".md") && f !== "README.md").map((f) => f.replace(/\.md$/, "")))
  : new Set();

// duplicate ids
const seen = new Set();
for (const t of workItems) {
  if (seen.has(t.meta.id)) err(t.file, `duplicate id ${t.meta.id}`);
  seen.add(t.meta.id);
}

for (const t of tasks) {
  const m = t.meta;
  for (const req of schema.required) if (!(req in m)) err(t.file, `missing required field: ${req}`);
  for (const [key, def] of Object.entries(schema.properties)) {
    if (!(key in m) || m[key] === null) continue;
    if (def.enum && !def.enum.includes(m[key])) err(t.file, `${key}: "${m[key]}" not in ${def.enum.join("|")}`);
    if (def.type === "array" && !Array.isArray(m[key])) err(t.file, `${key}: expected array`);
    if (def.pattern && typeof m[key] === "string" && !new RegExp(def.pattern).test(m[key]))
      err(t.file, `${key}: "${m[key]}" fails pattern`);
  }
  for (const key of Object.keys(m))
    if (!(key in schema.properties)) err(t.file, `unknown field: ${key}`);
  if (m.epic && !epicIds.has(m.epic) && !t.file.endsWith("_epic.md")) err(t.file, `epic ${m.epic} has no _epic.md`);
  for (const d of m.deps ?? [])
    if (!ids.has(d)) err(t.file, `dep "${d}" does not resolve to a task id`);
  for (const c of m.contexts ?? [])
    if (!packs.has(c)) err(t.file, `context pack "${c}" not found in contexts/`);
  const bodyLines = t.body.split("\n").length;
  if (bodyLines > 320) err(t.file, `body ${bodyLines} lines > 300-line cap`);
  if (m.status === "ready" && (!m.verify || m.verify.length === 0))
    err(t.file, `status ready requires a non-empty verify block`);
}

// caps: steering + packs
if (!existsSync(AGENTS_MD)) {
  // Remediation, not an ENOENT trace — and it must not abort before the entrypoint check below,
  // since a missing AGENTS.md is exactly when a dangling CLAUDE.md symlink matters most.
  errors.push(`AGENTS.md: missing — it is the real steering file; CLAUDE.md is a symlink to it`);
} else {
  const agentsLines = readFileSync(AGENTS_MD, "utf8").split("\n").length;
  if (agentsLines > 150) errors.push(`AGENTS.md: ${agentsLines} lines > 150 cap`);
}
for (const p of packs) {
  const lines = readFileSync(`${CONTEXTS_DIR}${p}.md`, "utf8").split("\n").length;
  if (lines > 160) errors.push(`contexts/${p}.md: ${lines} lines > 150 cap`);
}
if (packs.size > 12) errors.push(`contexts/: ${packs.size} packs > 12 cap`);

// entrypoint-symlink: CLAUDE.md must stay a symlink to AGENTS.md.
// Checked on the WORKING TREE, not the index: under a core.symlinks=false checkout the index
// still reads 120000 while CLAUDE.md is a 9-byte regular file containing "AGENTS.md", and a
// session auto-loading it gets those nine bytes instead of the Engineering OS — silently.
// CI note: actions/checkout preserves symlinks, so CI's own environment never produces the
// degraded form. The check still protects, because a core.symlinks=false contributor COMMITS a
// regular-file blob (mode 100644 lands in the index) and CI's lstat then sees a regular file.
try {
  if (!lstatSync(CLAUDE_MD).isSymbolicLink())
    errors.push(`entrypoint-symlink: CLAUDE.md is a regular file; it must stay a symlink to AGENTS.md — edit AGENTS.md instead`);
  else {
    const target = readlinkSync(CLAUDE_MD).replace(/^\.\//, ""); // ./AGENTS.md is equivalent
    if (target !== "AGENTS.md")
      errors.push(`entrypoint-symlink: CLAUDE.md points at "${target}"; it must point at AGENTS.md`);
  }
} catch (e) {
  errors.push(
    e.code === "ENOENT"
      ? `entrypoint-symlink: CLAUDE.md is missing; it must be a symlink to AGENTS.md`
      : `entrypoint-symlink: cannot stat CLAUDE.md (${e.code ?? e.message})`
  );
}

// agents-readme-exists: AGENTS.md step 5a points at this file; a rename would dangle silently.
// Runs before agents-table-sync so a missing README is a remediation, not an ENOENT trace.
const agentsReadmeOk = existsSync(AGENTS_README);
if (!agentsReadmeOk)
  errors.push(`agents-readme-exists: .claude/agents/README.md is missing — AGENTS.md step 5a points at it`);

if (existsSync(AGENTS_DIR)) {
  // sorted: readdirSync order is filesystem-dependent (APFS sorts, ext4 hashes), and error
  // order must be deterministic so output stays diffable.
  const dirents = readdirSync(AGENTS_DIR, { withFileTypes: true }).sort((a, b) => a.name.localeCompare(b.name));

  // Fail closed on subdirectories AND symlinks. The scan is deliberately non-recursive, so a
  // nested .claude/agents/<sub>/qa.md would otherwise escape agents-readonly entirely. isDirectory()
  // is false for a symlink, so a symlinked subdir (or file) pointing outside the repo would slip
  // through the same hole; one predicate closes both.
  for (const d of dirents)
    if (d.isDirectory() || d.isSymbolicLink())
      errors.push(`agents-table-sync: .claude/agents/${d.name} is a ${d.isSymbolicLink() ? "symlink" : "subdirectory"} — agent files must be regular files directly in .claude/agents/`);

  // Extension matched case-INsensitively: on a case-sensitive filesystem (Linux CI) a QA.MD
  // would otherwise be skipped silently, which is the same escape the name fold closes.
  const agentFiles = dirents
    .filter((d) => d.isFile() && /\.md$/i.test(d.name) && d.name.toLowerCase() !== "readme.md")
    .map((d) => d.name);
  const declared = new Map(); // lowercased name -> { name, file }

  for (const f of agentFiles) {
    let meta;
    try {
      ({ meta } = parseFrontmatter(readFileSync(`${AGENTS_DIR}${f}`, "utf8"), f));
    } catch {
      // Loud, not skipped: a typo'd fence is indistinguishable from no frontmatter at the parser.
      errors.push(`agents-table-sync: .claude/agents/${f} has missing or malformed frontmatter — README.md is the only permitted non-agent file here`);
      continue;
    }
    const name = String(meta.name ?? "");
    if (!name) { errors.push(`agents-table-sync: .claude/agents/${f} has no frontmatter name`); continue; }
    if (name !== f.replace(/\.md$/i, ""))
      errors.push(`agents-table-sync: .claude/agents/${f} declares name "${name}" — must match its filename stem`);
    // Two files whose names case-fold together would collapse to one Map entry, letting the
    // second escape the unlisted-agent check entirely.
    const key = name.toLowerCase();
    if (declared.has(key))
      errors.push(`agents-table-sync: .claude/agents/${f} declares "${name}", which case-folds onto "${declared.get(key).name}" in ${declared.get(key).file} — agent names must be unique case-insensitively`);
    else declared.set(key, { name, file: f });

    // agents-readonly (ADR-0008 reviewer identity). Case-insensitive: `subagent_type: qa` resolves
    // to a file declaring `name: QA`, so an exact-match filter would let it escape.
    if (REVIEW_AGENTS.includes(name.toLowerCase())) {
      // An ABSENT or empty `tools:` is the widest possible grant — the agent inherits the full
      // toolset, Write and Edit included. Requiring the key is the whole point of this check;
      // deleting the line is a likelier careless edit than adding a tool to it.
      if (meta.tools === undefined || meta.tools === null || String(meta.tools).trim() === "") {
        errors.push(`agents-readonly: .claude/agents/${f} declares no tools: — an omitted list inherits every tool, including Write and Edit; ADR-0008 requires an explicit list (allowed: ${[...READONLY_TOOLS].join(", ")})`);
      } else {
        const tools = String(meta.tools).split(",").map((s) => s.trim()).filter(Boolean);
        const bad = tools.filter((t) => !READONLY_TOOLS.has(t));
        if (bad.length)
          errors.push(`agents-readonly: .claude/agents/${f} requests ${bad.join(", ")} — ADR-0008 requires the two pipeline reviewers to request no write tools (allowed: ${[...READONLY_TOOLS].join(", ")}; note Bash is permitted for evidence-gathering and is NOT read-only)`);
      }
    }
  }

  // The two ADR-0008 reviewers must EXIST, not merely be well-formed if present.
  for (const want of REVIEW_AGENTS)
    if (!declared.has(want))
      errors.push(`agents-readonly: .claude/agents/${want}.md is missing — ADR-0008 makes it a mandatory pipeline reviewer`);

  // agents-table-sync: the README table is hand-maintained; keep it equal to the directory.
  if (agentsReadmeOk) {
    // Only lines inside a real table block count. Matching "any line containing a pipe" made
    // ordinary prose mentioning `reviewer.md` parse as a row.
    const tabled = new Map(); // lowercased subagent_type -> { name, fileCol }
    let inTable = false;
    for (const line of readFileSync(AGENTS_README, "utf8").split("\n")) {
      const trimmed = line.trim();
      if (!trimmed) { inTable = false; continue; }
      if (/^\|?[\s:|-]*-[\s:|-]*\|[\s:|-]*$/.test(trimmed) && trimmed.includes("-")) { inTable = true; continue; }
      if (!inTable) continue;
      const cols = trimmed.replace(/^\|/, "").replace(/\|$/, "").split("|").map((c) => c.trim().replace(/`/g, ""));
      if (cols.length < 2 || !/\.md$/i.test(cols[0])) continue;
      tabled.set(cols[1].toLowerCase(), { name: cols[1], fileCol: cols[0] });
    }
    for (const [key, { name, file }] of declared) {
      const row = tabled.get(key);
      if (!row) { errors.push(`agents-table-sync: agent "${name}" is not listed in .claude/agents/README.md's table`); continue; }
      if (row.name !== name)
        errors.push(`agents-table-sync: README table lists "${row.name}" but .claude/agents/${file} declares "${name}" — casing must match`);
      if (row.fileCol !== file)
        errors.push(`agents-table-sync: README table's File column says "${row.fileCol}" for "${name}", but it is declared in ${file}`);
    }
    for (const [key, { name }] of tabled)
      if (!declared.has(key))
        errors.push(`agents-table-sync: README table lists "${name}" but no .claude/agents/*.md declares it`);
  }
}

// ready-set report (informational)
const done = new Set(workItems.filter((t) => t.meta.status === "done").map((t) => t.meta.id));
const ready = workItems.filter(
  (t) => t.meta.status === "ready" && (t.meta.deps ?? []).every((d) => done.has(d))
);

if (errors.length) {
  console.error(`INVALID — ${errors.length} problem(s):`);
  for (const e of errors) console.error("  " + e);
  process.exit(1);
}
console.log(`OK: ${workItems.length} tasks, ${epicIds.size} epics, ${packs.size} packs, ${ready.length} ready-unblocked`);
