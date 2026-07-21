# Repository agents

Every `.md` file in this directory **with agent frontmatter** is a first-class subagent.
The harness registers it by its frontmatter `name`, and that name is the `subagent_type`
you invoke. (This README has no such frontmatter — it is documentation, not an agent.)

| File | `subagent_type` | Role |
|---|---|---|
| `reviewer.md` | `reviewer` | Architecture Reviewer (ADR-0008 pipeline) |
| `qa.md` | `qa` | Security/QA Reviewer (ADR-0008 pipeline) |
| `research.md` | `research` | Ecosystem investigation — instructed never to edit code |
| `docs.md` | `docs` | Developer-facing docs; instructed to write only under `docs/dev/` and `examples/` |

**Every scope limit above is an instruction to the agent, not an enforced sandbox.** `docs`
*requests* unscoped `Write`/`Edit`; the only `PreToolUse` hook guards
`docs/product/00-sources/**` and `decisions.md` and inspects only a tool's `file_path`. And
critically — see below — a `Bash` grant writes files, so **withholding `Write`/`Edit` does
not make an agent read-only.** Treat the column as intent and review what these agents did.

## The registration contract

1. **Name is the interface.** `subagent_type: "reviewer"` loads `reviewer.md`'s body as
   that agent's system prompt. Adding a file here with a new `name:` is the whole
   registration step — there is no manifest to update, but the table above is
   hand-maintained, so add a row.
2. **`tools:` is the requested grant, not a guarantee.** The effective toolset is
   harness-resolved and has been observed to be *narrower* than the frontmatter list. Never
   infer from this file that a given tool will be present — verify by observation. Do keep
   the list minimal: omit `Write`/`Edit` for any read-only role, since a grant that was
   never requested cannot be resolved into existence.
3. **Unknown names fail loudly.** Requesting an unregistered agent is a hard error that
   lists the available agents. There is no automatic fallback — a typo'd reviewer never
   degrades into some other reviewer.
4. **Repo agents shadow *namespaced* plugin agents.** Plugin agents carry a prefix
   (`kernel:reviewer`, `vercel:*`), so a bare name cannot reach them. Precedence against a
   *user-level* `~/.claude/agents/` file of the same bare name is **untested** — no such
   file existed when this was verified. If one is ever added, re-verify before trusting it.

## Kernel agents are prohibited

ADR-0008 owns this policy — read it there. In short: `kernel:*` agents were evaluated and
explicitly rejected for this repo, and no external, plugin, or machine-level reviewer may
stand in for `reviewer` or `qa`. They stay *visible* in the harness listing only because
they are installed at the machine level, outside this repo's control. Visibility is not
permission, and nothing currently *enforces* this — see O6a.

The ADR-0008 pipeline invokes `reviewer` and `qa` by name, directly. The older workaround
— running these files' text through a generic `general-purpose` runner — is obsolete. Note
that several completed task Outcomes still record it, because it was the house pattern when
they ran; those are history, not precedent.

## Verifying registration

Registration is a harness behavior, so verify by observation. Self-identification alone is
**not** sufficient — an agent handed this file's text by any other means recites it just as
well, which is precisely the retired workaround. Check the two things prompt text cannot fake:

- **Tool grant** — ask `reviewer` or `qa` to report whether a `Write` tool is *available*.
  It must report the tool is absent. A generic runner standing in has `Write`; tool grants
  are harness-enforced, so this is what prompt text cannot fake. Assert the tool's absence,
  not the agent's refusal to use it.

  This distinguishes a real repo agent from a stand-in. It does **not** establish that the
  agent cannot write: `reviewer` holds `Bash`, and `echo x > file` was verified to execute
  with no prompt. ADR-0008's "the two reviewers are read-only" is therefore a **behavioral
  constraint the agent observes, not a technical control** — no layer enforces it today.
  Do not cite the absent `Write` tool as if it were a sandbox. O6c tracks pinning what
  can be pinned.
- **Fail-fast** — request near-miss names where a substitution would plausibly occur.
  `reviewers` (plural) and `code-reviewer` (semantic neighbor) must hard-error and list the
  available agents. `Reviewer` (case) is the exception: matching is **case-insensitive**, so
  it resolves — but it resolves to *this* repo's `reviewer`, verified by the tool-grant and
  identity probes above. That is a fold, not a substitution. Anything that resolves to a
  plugin, or to an agent whose grant is wider than its file requests, is the failure.

Neither check detects a *stale* prompt (file edited, old version still served). See O6c.

## Adding an agent

Create `<name>.md` here with `name`, `description`, and a minimal `tools:` list, where
`name` matches the filename stem. Add a row to the table above. Then run both checks. If
the agent participates in a mandatory pipeline, say so in ADR-0008 too — this directory
defines agents, not policy.
