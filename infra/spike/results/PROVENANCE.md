# Results provenance (evidence-integrity record)

Raw logs are append-only captures; this file states exactly which process wrote
which lines, per the review finding that unattributed appends defeat committed
evidence.

## pdcsi.log
Entirely produced by `run-pdcsi.sh` (committed), single run 2026-07-19 ~15:51–15:55 UTC.

## pdcsi-phase2.log
- Lines up to and including the second `pitr_to_new_s=903.8` block: produced by
  `run-pdcsi-phase2.sh run` as then committed. Two of its outputs were WRONG and
  are superseded:
  - `WARN no WAL archived in 300s` — false negative from a glob missing barman's
    nested `serverName/` dir (`…/spike-a/spike-a/wals/`); `ContinuousArchiving`
    was `True` throughout. The glob is fixed in the committed script and the
    verification listing now lands in the log on every run.
  - `pitr_to_new_s=903.8` — the `targetTime` PITR hang (finding F4), a timeout,
    not a restore time.
- The final lines (`pitr_to_new_s=55.2 …`, `restored: post-backup marker=1, load
  rows=1320000`) were appended by ad-hoc session commands (2026-07-19
  ~16:29–16:56 UTC) that diagnosed and completed step E: an on-demand `Backup`
  (spike-a-base, `phase: completed`, beginWal=endWal=…1D, stoppedAt
  2026-07-19T16:29:33Z), a `pg_switch_wal` marker, then a **restore-to-latest**
  cluster (no recoveryTarget) timed with the same monotonic-clock method as the
  script. Those exact steps are NOW COMMITTED as `manifests/spike-a-backup.yaml`
  + `manifests/spike-restore-latest.yaml.tpl` + the rewritten step E of
  `run-pdcsi-phase2.sh`, so a rerun reproduces the measurement path that
  produced 55.2 s.
- `rpo_measured_s=2` from the original run is superseded by the honest labeling
  in the rewritten step D: the number is the **unarchived window** at a
  hypothetical failure instant; the RPO figure that A1.3 consumes is the
  **300 s bound by construction** (`archive_timeout`) — no destructive kill test
  was run (deferred to T1.2's harness).

## provisioning-failures.log
Captured post-hoc (2026-07-19 ~17:10 UTC) from GCP's operation history via the
gcloud commands shown in the file header — the `ZONE_RESOURCE_POOL_EXHAUSTED`
insert errors for both the n2+local-SSD node (spike-cnpg) and the n2 PD-only
node (spike-db). Single-zone (us-central1-a), single-day observation.

## teardown.log
Produced by `run-pdcsi-teardown.sh` (committed, idempotent), run 2026-07-19
17:17 UTC after ad-hoc teardown — every step reports "already gone" and the
four orphan-sweep listings are empty, which is simultaneously the $0-state
evidence and the idempotency proof.
