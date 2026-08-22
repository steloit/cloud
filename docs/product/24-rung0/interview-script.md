# Developer interview script — Rung 0 (v2, 2026-08-22 · positioning v2 · `docs/plan/positioning-v2.md`)

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

- What are you building, and how much of it is AI-written these days?
- What's running in production today, and who deploys it?
- Who sees the cloud bill? *(Out of segment only if there is no
  application and no intention to run one. Not knowing infrastructure
  is the segment, not a disqualification — see the pitch's "who this is
  for".)*

## 1 · Current state (10 min — their life, not our idea)

**The gap (lead here):**
- Think of the last thing you shipped. How long between "it works" and
  "it's live"? What were you actually doing in that time?
- Walk me through everything you had to set up before real users could
  hit it. *(Listen for: VM sizing, Kubernetes, load balancer, DNS/TLS,
  database, env vars, backups, monitoring. Count them. Don't help.)*
- Which of those did you understand, and which did you copy from a
  tutorial or ask an AI for? *(Zero judgement in the voice. This is the
  central data point of the v2 thesis.)*
- Is there anything you've built and *not* deployed? Why not?

**The rest:**
- Walk me through your last deploy that scared you. What made it scary?
- What happened the last time the cloud bill surprised you? Walk me
  through the hour after you opened it.
- Tell me about your staging environment. When did it last disagree
  with production, and what did that cost?

## 2 · Quantify the scar (8 min)

- How many hours went into infrastructure setup on that last project —
  actual hours, roughly? Whose hours?
- What do you pay today for the infrastructure under it — and do you
  know without looking? *(The pause IS the data point.)*
- How much engineer time went into that incident/bill/staging mess?
- Have you tried to fix this? What did you try, what did it cost, why
  did it stall? *(No attempt to fix = pain isn't real. Log it.)*
- If nothing changes, what does this cost you over the next year?

## 3 · The reveal (5 min — only now, wedge first)

Show, in order: **repo → recommended infrastructure → projected cost →
accept → running** (on their repository if they'll give you one) → the
composer pricing THEIR stack live, with a cap refusing an over-bound
estimate → the alpha path honestly (per the pitch's candor block,
including the durability line and the staging of application
understanding). Previews-with-masked-data only if they pull. Then stop
talking and count to five.

Correct two misreadings on the spot if they appear, then keep going:
that we review or test their code (we don't — we work out what it
needs), and that the price is fixed (a projection is a range; the cap
is the guarantee).

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
segment fit (y/n) · **the gap: hours between working and live, and what
filled them** · **which infrastructure pieces they didn't understand,
verbatim** · the scar, verbatim quote · $ / hours quantified · what they
tried already · price anchor they volunteered · reaction to founding
price, verbatim · ladder rung reached (0–4) · contamination point (when
the pitch started) · follow-up owed.

## Scoring (weekly synthesis, against the §4 gate)

- **Strong:** quantified pain + rung ≥ 2 (LOI or better). A quantified
  *gap* (hours lost between working and live) counts as quantified pain
  — that is the v2 thesis being tested, and it is the number to watch.
- **Interested:** quantified pain, rung 1 — nurture, don't count.
- **Polite:** compliments, no numbers, no commitment — counts as zero.
- Gate math: the sprint is justified by LOIs and deposits, not by the
  interview count. If 30 conversations produce < 5 strong, the finding
  is the finding — per §4, that is the demand-validation answer, and
  building anyway is the named trap.
