# Repository agents

Every `.md` file in this directory is a first-class subagent. The harness registers it
by its frontmatter `name`, and that name is the `subagent_type` you invoke.

| File | `subagent_type` | Role |
|---|---|---|
| `reviewer.md` | `reviewer` | Architecture Reviewer (ADR-0008 pipeline, read-only) |
| `qa.md` | `qa` | Security/QA Reviewer (ADR-0008 pipeline, read-only) |
| `research.md` | `research` | Ecosystem investigation — never edits code |
| `docs.md` | `docs` | Developer-facing docs; writes only under `docs/dev/` and `examples/` |

## The registration contract

1. **Name is the interface.** `subagent_type: "reviewer"` loads `reviewer.md`'s body as
   that agent's system prompt, and grants exactly its frontmatter `tools:`. Adding a file
   here with a new `name:` is the whole registration step — there is no manifest to update.
2. **Repo agents shadow plugins.** A bare name resolves to this directory. Machine-level
   plugin agents are namespaced (`kernel:reviewer`, `vercel:*`) and can never be reached by
   a bare name, so they cannot silently stand in for a repo agent.
3. **Unknown names fail loudly.** Requesting an unregistered agent is a hard error that
   lists the available agents. There is no automatic fallback — a typo'd reviewer never
   degrades into some other reviewer.

## Kernel agents are prohibited

`kernel:*` agents were evaluated and **explicitly rejected for this repo**
(`docs/plan/kernel-workflow-review.md`; ADR-0008 "Reviewer identity is fixed and
repo-native"). They remain visible in the harness listing because they are installed at
the machine level, outside this repo's control — visibility is not permission. Invoking
one for a review is the same class of process violation as skipping a reviewer, and the
review it produces does not count.

The ADR-0008 pipeline invokes `reviewer` and `qa` by name, directly. The older workaround
— running these files' text through a generic `general-purpose` runner — is obsolete;
do not reintroduce it.

## Verifying registration

Registration is a harness behavior, so verify it by observation, not by assertion:

- **Resolution** — invoke the agent and ask it to answer from its own system prompt only
  (e.g. `reviewer` must self-identify as the *Steloit Reviewer Agent* and reproduce its
  `VERDICT:` template; `qa` must give `GAPS:`/`FUZZ:`/`REGRESSION:` and cite
  `docs/product/16-qa/qa.md`). A generic answer means the file did not load.
- **Fail-fast** — request a name that does not exist and confirm you get an error listing
  the available agents, not a substitution.

## Adding an agent

Create `<name>.md` here with `name`, `description`, and a minimal `tools:` list — grant
only what the role needs, and omit `Edit`/`Write` for any read-only role. Then run the two
checks above. If the agent participates in a mandatory pipeline, say so in ADR-0008 too;
this directory defines agents, not policy.
