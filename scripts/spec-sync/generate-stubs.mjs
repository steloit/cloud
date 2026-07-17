#!/usr/bin/env node
// ONE-TIME seed: transform data/{epics,issues}.json + .state/issue-map.json into tasks/** stubs.
// Idempotent: skips files that already exist (enrichment is never overwritten).
import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import { join } from "node:path";
import { serializeFrontmatter, TASKS_DIR } from "./lib.mjs";

const here = new URL(".", import.meta.url).pathname;
const epics = JSON.parse(readFileSync(join(here, "data/epics.json"), "utf8"));
const issues = JSON.parse(readFileSync(join(here, "data/issues.json"), "utf8"));
const issueMap = JSON.parse(readFileSync(join(here, ".state/issue-map.json"), "utf8"));

const EPIC_DIR = {
  E0: "e0-setup", E1: "e1-substrate", E2: "e2-identity", E3: "e3-provisioning",
  E4: "e4-deploy", E5: "e5-cli", E6: "e6-observe-metering", E7: "e7-auth",
  E8: "e8-console", E9: "e9-data-layer", E10: "e10-observability", E11: "e11-billing",
  E12: "e12-governance", E13: "e13-ai-plane", E14: "e14-data-plane", EQA: "eqa", EOPS: "eops",
};
const EPIC_CONTEXTS = {
  E0: [], E1: ["provisioning"], E2: ["api-conventions", "rbac", "events-spine"],
  E3: ["provisioning", "api-conventions", "canon-testing"], E4: ["provisioning", "events-spine"],
  E5: ["api-conventions"], E6: ["events-spine", "billing"], E7: ["rbac", "api-conventions"],
  E8: ["frontend-console", "api-conventions"], E9: ["provisioning", "api-conventions"],
  E10: ["events-spine", "api-conventions"], E11: ["billing", "canon-testing", "api-conventions"],
  E12: ["rbac", "api-conventions"], E13: ["ai-plane", "rbac"],
  E14: ["api-conventions", "provisioning"], EQA: ["canon-testing"], EOPS: ["provisioning"],
};

const sprintNum = (s) => {
  const m = /Sprint (\d+)/.exec(s ?? "");
  return m ? Number(m[1]) : null;
};
let created = 0, skipped = 0;

const write = (dir, name, meta, body) => {
  mkdirSync(dir, { recursive: true });
  const p = join(dir, name);
  if (existsSync(p)) { skipped++; return; }
  writeFileSync(p, serializeFrontmatter(meta) + "\n" + body.trim() + "\n");
  created++;
};

for (const e of epics) {
  const dir = join(TASKS_DIR, EPIC_DIR[e.key]);
  write(dir, "_epic.md", {
    id: e.key, title: e.title, epic: e.key, status: "stub", phase: e.phase,
    priority: e.priority.toLowerCase(), sprint: sprintNum(e.sprint ?? e.milestone),
    estimate: `${e.estimate}ew`, deps: e.deps ?? [], issue: issueMap[e.key] ?? null,
    labels: e.labels ?? [], module: e.module, contexts: [], owner: "founders",
  }, `## Scope\n\n${e.body}\n\n> Epic tracking file. Work items live beside it; shared design goes here as the epic starts.`);
}

for (const i of issues) {
  const dir = join(TASKS_DIR, EPIC_DIR[i.epic]);
  const [summary, ac] = i.body.split(/\n\*\*AC:\*\*\s*/s);
  const body = [
    `## Goal\n\n${i.title.replace(/^[^·]+·\s*/, "")}`,
    `## Summary\n\n${summary.trim()}`,
    ac ? `## Acceptance criteria\n\n- [ ] ${ac.trim()}` : null,
    `> **Stub** — run the spec-author skill to enrich to \`ready\` before starting. Plan reference: docs/plan/implementation-plan.md §5 ${i.epic}.`,
  ].filter(Boolean).join("\n\n");
  write(dir, `${i.key}.md`, {
    id: i.key, title: i.title.replace(/^[^·]+·\s*/, ""), epic: i.epic, status: "stub",
    phase: i.phase, priority: i.priority.toLowerCase(), sprint: sprintNum(i.sprint),
    estimate: `${i.estimate}ew`, deps: i.deps ?? [], issue: issueMap[i.key] ?? null,
    labels: i.labels ?? [], module: i.module, contexts: EPIC_CONTEXTS[i.epic] ?? [],
    files: [], verify: [], owner: "agent",
  }, body);
}

console.log(`stubs: ${created} created, ${skipped} already existed`);
