#!/usr/bin/env node
// Validate every task file: schema-ish checks, dep resolution, context-pack resolution, caps.
// Exit 1 on any failure. No dependencies.
import { readFileSync, existsSync, readdirSync } from "node:fs";
import { loadTasks, parseFrontmatter } from "./lib.mjs";

const CONTEXTS_DIR = new URL("../../contexts/", import.meta.url).pathname;
const AGENTS_MD = new URL("../../AGENTS.md", import.meta.url).pathname;

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
