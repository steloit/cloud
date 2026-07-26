# Developer interview script — Rung 0 (v1, 2026-07-13)

The instrument for the 30–50 conversations (INF-001 §4). Goal:
**evidence of demand, not validation theater.** The gate this feeds:
design-partner LOIs and pre-orders — costly commitments. Compliments
are logged as zero.

Ground rules (Mom-Test discipline, enforced on ourselves):
1. **No pitch for the first 20 minutes.** The moment you describe
   Steloit, everything after is contaminated — mark it in the log.
2. **Past behavior only.** "Tell me about the last time…" — never
   "would you use…". Hypothetical enthusiasm is not data.
3. **Money questions are specific.** What did it cost, in hours or
   dollars, last month — not "is this painful."
4. **The only positive signals are costly:** time scheduled, LOI
   signed, deposit paid, an intro made. Everything else is politeness.

---

## 0 · Screener (2 min)

- What do you run in production today, and who deploys it?
- Who sees the cloud bill? *(If neither answer is "me" → thank, end,
  log as out-of-segment. Do not pitch.)*

## 1 · Current state (10 min — their life, not our idea)

- Walk me through your last deploy that scared you. What made it scary?
- When a PR touches data, what does the reviewer actually test
  against? *(Listen for: toy fixtures, shared staging, "we're careful.")*
- Tell me about your staging environment. When did it last disagree
  with production, and what did that cost?
- What happened the last time the cloud bill surprised you? Walk me
  through the hour after you opened it.

## 2 · Quantify the scar (8 min)

- How much engineer time went into that incident/bill/staging mess —
  actual hours, roughly?
- What do you pay today for preview/staging infrastructure — and do
  you know without looking? *(The pause IS the data point.)*
- Have you tried to fix this? What did you try, what did it cost, why
  did it stall? *(No attempt to fix = pain isn't real. Log it.)*
- If nothing changes, what does this cost you over the next year?

## 3 · The reveal (5 min — only now, wedge first)

Show, in order: the PR-comment wedge (masked branch · $0.07/day) → the
composer pricing THEIR stack live → the alpha path honestly (per the
pitch's candor block, including the durability line). Then stop
talking and count to five.

- What would you expect this to cost? *(Anchor them first, not us.)*
- Then, and only then: founding pricing. Watch the face, log the
  reaction verbatim.

## 4 · The commitment ladder (5 min — the actual test)

Ask up the ladder until a real "no":
1. "Can I get 30 minutes with the teammate who felt this most?"
2. "Would you sign a design-partner LOI — named team, named app,
   intent to run a real workload?"
3. "Can we put your onboarding on the calendar against our day-90?"
4. "Founding pricing is locked for deposits — would you put money down?"

A "yes" that survives the calendar invite is a yes. A "yes" without a
date is a no, logged politely.

## 5 · Log (immediately after, 5 min — one file per interview)

`24-rung0/interviews/YYYY-MM-DD-<name>.md`:
segment fit (y/n) · the scar, verbatim quote · $ / hours quantified ·
what they tried already · price anchor they volunteered · reaction to
founding price, verbatim · ladder rung reached (0–4) · contamination
point (when the pitch started) · follow-up owed.

## Scoring (weekly synthesis, against the §4 gate)

- **Strong:** quantified pain + rung ≥ 2 (LOI or better).
- **Interested:** quantified pain, rung 1 — nurture, don't count.
- **Polite:** compliments, no numbers, no commitment — counts as zero.
- Gate math: the sprint is justified by LOIs and deposits, not by the
  interview count. If 30 conversations produce < 5 strong, the finding
  is the finding — per §4, that is the demand-validation answer, and
  building anyway is the named trap.
