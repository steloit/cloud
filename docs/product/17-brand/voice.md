# Voice & writing guide

For every word the frames don't already own. Existing microcopy is spec — copied verbatim, never paraphrased. This guide governs **new** words: the next error message, empty state, email, doc page, release note. The persona is the **candid engineer** (brand.md): candid · composed · exact · structural. The one test, from the constitution: *what does the reader know after this sentence?*

## The register, in five rules

1. **State, don't perform.** Facts in indicative mood; no exclamation marks in product surfaces; the drama budget is spent only on genuine failure. "Provisioning failed at step 3: network attach timed out" — not "Oops! Something went wrong 😢".
2. **Numbers are the voice.** Where a number exists, use it — exact, honest (`~$9` when approximate, `$1.62` when computed), in mono when copyable. "Egress at 87 of 100 GB — at this pace, ~$1.62 overage this month" *is* Steloit talking.
3. **Every failure names a way forward** (A7 is the bar). Structure: what happened → why/where (named policy, named role, named step) → what to do next. No dead ends, no "contact support" as a first resort, always a reference id in mono for 5xx.
4. **Name the cause, own the contract.** "Provisioning paused — payment failed 8 days ago; retries continue through day 21; running services untouched." The dunning email and the banner say the same thing because the contract was printed before it was needed.
5. **Calculators, not sales.** Recommendations show the math and include "do nothing" when it's the cheaper answer. The assistant never nags, never upsells; neither does any sentence anywhere.

## Vocabulary discipline

- **Banned:** blazingly, seamless, magical, powerful, effortless, "simply/just" (if it were simple they wouldn't be reading), delightful, oops, whoops, "we're sorry for any inconvenience" (either say what happened or say nothing).
- **Required precision:** the glossary's terms exactly (18-philosophy/glossary.md) — a branch is not a copy, silence is not mute, `ready` not "running". New nouns register in the glossary before first use.
- **Casing:** sentence case everywhere (headings, buttons, labels); products and primitives capitalized only when named as such.
- **Mono means copyable** — ids, commands, URLs, env vars, money in tables. If it can be pasted into a terminal, it is mono; if it's mono, it had better paste correctly.

## The registers (one voice, four distances)

- **UI microcopy** — tersest; the words are load-bearing spec ("shown once", "no silent limbo"). New microcopy states the contract at the moment of doubt: the best examples put the rule *inside* the surface ("billing starts at ready, never at click").
- **Docs** — structured by developer question, not product noun (the question-ownership rule applied to writing). Every task page ends with the CLI equivalent. Present tense, second person, no "we".
- **Marketing** — proof over claims: real product UI, canon numbers, the enemy named (bill shock, silent limbo, the mystery deploy). First person plural allowed; superlatives still banned — the screenshot makes the argument.
- **Email/transactional** — see 22-emails/. An email is an event with a deep link; it earns its interruption or it isn't sent.

## Before / after

| Instead of | Write |
|---|---|
| "Something went wrong. Please try again later." | "Couldn't reach the API (request `req_8f2a1`). Retrying automatically — your services are unaffected." |
| "Upgrade to unlock more power!" | "2 cells need Business ($99/mo). Staying on Pro keeps both cells read-only." |
| "Your database is now blazingly fast!" | "p95 431 ms — down 47% since #143." |
| "Are you sure you want to delete?" | "Deleting db-main breaks api and worker — knowingly. A final backup stays restorable for 30 days. Type **db-main** to confirm." |
