# Database Load After-Change Notes

## Purpose And Safety Notes

This document is the post-change companion to `db-load-baseline.md`. Use it after deploying the DB load-reduction work to record sanitized evidence and rollback criteria.

Safety rules:

- Do not print, paste, or store `DATABASE_URL` or credentials.
- Record only sanitized counts, query IDs, and high-level interpretations.
- Do not use `EXPLAIN ANALYZE` against live production traffic paths unless the query is known to be safe and bounded.

## After-Change Capture Steps

Run from `server/` after deployment:

1. Confirm the app is healthy and the migration set applied cleanly.
2. Re-run the same baseline SQL from `db-load-baseline.md` for `pg_stat_statements`, `pg_stat_activity`, and cache hit snapshots.
3. Capture the same query ID groupings before comparing them to the baseline.
4. Note whether the connection URL is direct or pooler without exposing the URL itself.
5. Record any relevant worker or API regressions observed during the capture window.

## Expected Improvements

Compare results against the baseline and note whether these moved in the expected direction:

- lower idle connection count after pool tuning
- lower call volume for assignment polling queries during idle periods
- lower rows scanned by completion and reminder workers
- fewer repeated rider dispatch lookup queries per worker tick
- fewer `COUNT(*)` plus `OFFSET` calls on changed list endpoints

## Reminder Backfill Note

The reminder-job migration includes a backfill for existing bookings with `status = 'assigned'`, a non-null `scheduled_start`, and a future `scheduled_start`. It inserts both `reminder_24h` and `reminder_2h` jobs and uses the same conflict behavior as the trigger so repeated application stays duplicate-safe.

## Rollback Criteria

Roll back the change if any of the following appear after deployment:

- increased assignment latency beyond the accepted SLA
- missed completion or reminder events
- increased API errors from pool exhaustion
- duplicate notification or ride dispatch behavior

## Capture Template

```markdown
## Capture Metadata

- Captured at UTC:
- Captured by:
- Environment: Supabase/Postgres
- Connection mode: direct or pooler, if known without revealing URL

## Sanitized Findings

- Top total-time query IDs:
- Top call-volume query IDs:
- Top row-volume query IDs:
- Connection snapshot:
- Cache hit baseline:
- Reminder backfill check:
- Rollback criteria status:
```
