#!/usr/bin/env node
// Validate every task file: schema-ish checks, dep resolution, context-pack resolution, caps.
// Exit 1 on any failure. No dependencies.
import { readFileSync, existsSync, readdirSync, lstatSync, readlinkSync } from "node:fs";
import { loadTasks, parseFrontmatter } from "./lib.mjs";

const CONTEXTS_DIR = new URL("../../contexts/", import.meta.url).pathname;
const AGENTS_MD = new URL("../../AGENTS.md", import.meta.url).pathname;
const CLAUDE_MD = new URL("../../CLAUDE.md", import.meta.url).pathname;
const AGENTS_DIR = new URL("../../.claude/agents/", import.meta.url).pathname;
const AGENTS_README = `${AGENTS_DIR}README.md`;
// ADR-0008: the two pipeline reviewers must not request write tools.
const REVIEW_AGENTS = ["reviewer", "qa"];
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
const agentsLines = readFileSync(AGENTS_MD, "utf8").split("\n").length;
if (agentsLines > 150) errors.push(`AGENTS.md: ${agentsLines} lines > 150 cap`);
for (const p of packs) {
  const lines = readFileSync(`${CONTEXTS_DIR}${p}.md`, "utf8").split("\n").length;
  if (lines > 160) errors.push(`contexts/${p}.md: ${lines} lines > 150 cap`);
}
if (packs.size > 12) errors.push(`contexts/: ${packs.size} packs > 12 cap`);

// entrypoint-symlink: CLAUDE.md must stay a symlink to AGENTS.md.
// Checked on the WORKING TREE, not the index: under a core.symlinks=false checkout the index
// still reads 120000 while CLAUDE.md is a 9-byte regular file containing "AGENTS.md", and a
// session auto-loading it gets those nine bytes instead of the Engineering OS — silently.
try {
  const st = lstatSync(CLAUDE_MD);
  if (!st.isSymbolicLink())
    errors.push(`entrypoint-symlink: CLAUDE.md is a regular file; it must stay a symlink to AGENTS.md — edit AGENTS.md instead`);
  else if (readlinkSync(CLAUDE_MD) !== "AGENTS.md")
    errors.push(`entrypoint-symlink: CLAUDE.md points at "${readlinkSync(CLAUDE_MD)}"; it must point at AGENTS.md`);
} catch {
  errors.push(`entrypoint-symlink: CLAUDE.md is missing; it must be a symlink to AGENTS.md`);
}

// agents-readme-exists: AGENTS.md step 5a points at this file; a rename would dangle silently.
// Runs before agents-table-sync so a missing README is a remediation, not an ENOENT trace.
const agentsReadmeOk = existsSync(AGENTS_README);
if (!agentsReadmeOk)
  errors.push(`agents-readme-exists: .claude/agents/README.md is missing — AGENTS.md step 5a points at it`);

if (existsSync(AGENTS_DIR)) {
  const agentFiles = readdirSync(AGENTS_DIR).filter((f) => f.endsWith(".md") && f !== "README.md");
  const declared = new Map(); // name -> filename

  for (const f of agentFiles) {
    let meta;
    try {
      ({ meta } = parseFrontmatter(readFileSync(`${AGENTS_DIR}${f}`, "utf8"), f));
    } catch {
      // Loud, not skipped: a typo'd fence is indistinguishable from no frontmatter at the parser.
      errors.push(`agents-table-sync: .claude/agents/${f} has missing or malformed frontmatter`);
      continue;
    }
    const name = meta.name;
    if (!name) { errors.push(`agents-table-sync: .claude/agents/${f} has no frontmatter name`); continue; }
    if (name !== f.replace(/\.md$/, ""))
      errors.push(`agents-table-sync: .claude/agents/${f} declares name "${name}" — must match its filename stem`);
    declared.set(name, f);

    // agents-readonly (ADR-0008 reviewer identity): the two reviewers request no write tools.
    if (REVIEW_AGENTS.includes(name)) {
      const tools = String(meta.tools ?? "").split(",").map((s) => s.trim()).filter(Boolean);
      const bad = tools.filter((t) => !READONLY_TOOLS.has(t));
      if (bad.length)
        errors.push(`agents-readonly: .claude/agents/${f} requests ${bad.join(", ")} — ADR-0008 requires the two pipeline reviewers to be read-only (allowed: ${[...READONLY_TOOLS].join(", ")})`);
    }
  }

  // agents-table-sync: the README table is hand-maintained; keep it equal to the directory.
  if (agentsReadmeOk) {
    const tabled = new Set();
    for (const line of readFileSync(AGENTS_README, "utf8").split("\n")) {
      if (!line.startsWith("|")) continue;
      const cols = line.split("|").slice(1, -1).map((c) => c.trim().replace(/`/g, ""));
      if (cols.length < 2 || !cols[0].endsWith(".md")) continue; // header + separator rows
      tabled.add(cols[1]);
    }
    for (const name of declared.keys())
      if (!tabled.has(name))
        errors.push(`agents-table-sync: agent "${name}" is not listed in .claude/agents/README.md's table`);
    for (const name of tabled)
      if (!declared.has(name))
        errors.push(`agents-table-sync: README table lists "${name}" but no .claude/agents/${name}.md declares it`);
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
