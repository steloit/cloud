// Shared helpers for spec-sync: frontmatter parse/serialize, task discovery. No dependencies.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { join } from "node:path";

export const TASKS_DIR = new URL("../../tasks/", import.meta.url).pathname;

/** Minimal YAML subset parser for our frontmatter: scalars, flow arrays, block arrays of scalars. */
export function parseFrontmatter(text, file = "?") {
  const m = /^---\n([\s\S]*?)\n---\n?/.exec(text);
  if (!m) throw new Error(`${file}: missing frontmatter`);
  const meta = {};
  const lines = m[1].split("\n");
  let pendingKey = null;
  for (const raw of lines) {
    const line = raw.replace(/\s+#.*$/, "").trimEnd();
    if (!line.trim()) continue;
    const blockItem = /^\s+-\s*(.+)$/.exec(line);
    if (blockItem && pendingKey) {
      meta[pendingKey].push(coerce(blockItem[1]));
      continue;
    }
    const kv = /^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$/.exec(line);
    if (!kv) continue;
    const [, key, valueRaw] = kv;
    pendingKey = null;
    if (valueRaw === "") {
      meta[key] = [];
      pendingKey = key;
    } else if (valueRaw.startsWith("[")) {
      const inner = valueRaw.replace(/^\[|\]$/g, "").trim();
      meta[key] = inner === "" ? [] : inner.split(",").map((s) => coerce(s));
    } else {
      meta[key] = coerce(valueRaw);
    }
  }
  return { meta, body: text.slice(m[0].length) };
}

function coerce(s) {
  const v = String(s).trim().replace(/^["']|["']$/g, "");
  if (/^-?\d+$/.test(v)) return Number(v);
  if (v === "null") return null;
  return v;
}

export function serializeFrontmatter(meta) {
  const order = ["id","title","epic","status","phase","priority","sprint","estimate","deps","issue",
    "labels","module","contexts","files","apis","tables","events","tests","verify","owner"];
  const lines = [];
  for (const k of order) {
    if (!(k in meta)) continue;
    const v = meta[k];
    if (Array.isArray(v)) {
      if (v.length === 0) lines.push(`${k}: []`);
      else if (v.every((x) => String(x).length < 60 && !String(x).includes(",")) && v.length <= 6)
        lines.push(`${k}: [${v.join(", ")}]`);
      else { lines.push(`${k}:`); for (const x of v) lines.push(`  - ${x}`); }
    } else lines.push(`${k}: ${v}`);
  }
  return `---\n${lines.join("\n")}\n---\n`;
}

/** All task files (excluding _template, _epic, _archive). Returns [{file, meta, body}]. */
export function loadTasks(includeEpics = false) {
  const out = [];
  const walk = (dir) => {
    for (const name of readdirSync(dir)) {
      const p = join(dir, name);
      if (statSync(p).isDirectory()) {
        if (name === "_archive") continue;
        walk(p);
      } else if (name.endsWith(".md") && name !== "_template.md") {
        if (name === "_epic.md" && !includeEpics) continue;
        const text = readFileSync(p, "utf8");
        out.push({ file: p, ...parseFrontmatter(text, p) });
      }
    }
  };
  walk(TASKS_DIR);
  return out;
}
