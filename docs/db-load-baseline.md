# Database Load Baseline

## Purpose and Safety Notes

This runbook captures a repeatable Supabase/Postgres load baseline before database-load reduction work. It is intended to be executable by agents from a shell with `psql` and `DATABASE_URL`, without requiring Supabase dashboard-only manual steps.

Safety rules:

- Do not print, paste, or store `DATABASE_URL` or any credentials.
- Prefer aggregate and fingerprint-safe `pg_stat_statements` fields: `queryid`, `calls`, `mean_exec_time`, `total_exec_time`, `rows`, `shared_blks_hit`, `shared_blks_read`, and `temp_blks_written`.
- Do not include raw `query` text in primary baseline evidence. Query text can contain literals or sensitive business context depending on normalization and extension settings.
- Summarize results by `queryid` and metrics only. If a result needs product interpretation, map it to an in-repo code path without pasting production data.
- `EXPLAIN ANALYZE` executes the query being explained. Do not use it against production traffic paths during baseline capture unless the query is known to be safe and bounded.
- Supabase projects often have `pg_stat_statements` available, but permissions can vary. Some checks may require a role with stats visibility such as `pg_read_all_stats`.
- Supabase pooler and direct connection modes can show different connection behavior. Record whether the URL targets the pooler or a direct database endpoint without exposing the URL.

## Prerequisites

- Run commands from `server/` unless noted otherwise.
- `DATABASE_URL` is present in the environment. Check presence without printing it:

```powershell
Test-Path Env:DATABASE_URL
```

- `psql` is installed and available on `PATH`:

```powershell
Get-Command psql -ErrorAction SilentlyContinue
```

- Use `ON_ERROR_STOP=1` for every `psql` command so failures are explicit:

```powershell
psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -c "SELECT 1;"
```

## Extension Check SQL

```sql
SELECT extname
FROM pg_extension
WHERE extname = 'pg_stat_statements';
```

Expected result: one row with `pg_stat_statements`. If no row appears, confirm Supabase project settings and whether extension creation is allowed for the project before adding any migration.

## Top Queries by Total Execution Time SQL

```sql
SELECT
  queryid,
  calls,
  mean_exec_time,
  total_exec_time,
  rows,
  shared_blks_hit,
  shared_blks_read,
  temp_blks_written
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY total_exec_time DESC
LIMIT 25;
```

Use this to find statements consuming the most cumulative execution time.

## Top Queries by Call Volume SQL

```sql
SELECT
  queryid,
  calls,
  mean_exec_time,
  total_exec_time,
  rows,
  shared_blks_hit,
  shared_blks_read,
  temp_blks_written
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY calls DESC
LIMIT 25;
```

Use this to find high-frequency polling, worker, or API statements even when each call is individually cheap.

## Top Queries by Row Volume SQL

```sql
SELECT
  queryid,
  calls,
  mean_exec_time,
  total_exec_time,
  rows,
  shared_blks_hit,
  shared_blks_read,
  temp_blks_written
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
ORDER BY rows DESC
LIMIT 25;
```

Use this to find unbounded scans, pagination issues, or fanout paths that process too many rows.

## Connection Snapshot SQL using `pg_stat_activity`

```sql
SELECT
  state,
  wait_event_type,
  wait_event,
  count(*) AS connection_count
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY state, wait_event_type, wait_event
ORDER BY connection_count DESC, state, wait_event_type, wait_event;
```

Optional role/application breakdown, only if the result does not expose sensitive tenant-specific naming:

```sql
SELECT
  usename,
  application_name,
  state,
  count(*) AS connection_count
FROM pg_stat_activity
WHERE datname = current_database()
GROUP BY usename, application_name, state
ORDER BY connection_count DESC, usename, application_name, state;
```

Use connection snapshots to compare direct database connections with Supabase pooler behavior and to identify idle footprint before pool tuning.

## Cache Hit Baseline SQL

```sql
SELECT
  sum(heap_blks_hit) AS heap_blks_hit,
  sum(heap_blks_read) AS heap_blks_read,
  round(
    100 * sum(heap_blks_hit)::numeric / NULLIF(sum(heap_blks_hit) + sum(heap_blks_read), 0),
    2
  ) AS heap_cache_hit_percent,
  sum(idx_blks_hit) AS idx_blks_hit,
  sum(idx_blks_read) AS idx_blks_read,
  round(
    100 * sum(idx_blks_hit)::numeric / NULLIF(sum(idx_blks_hit) + sum(idx_blks_read), 0),
    2
  ) AS index_cache_hit_percent
FROM pg_statio_user_tables;
```

Use this as a coarse cache baseline. Low hit ratios can indicate large scans, missing indexes, cold cache, or workload changes; interpret alongside top query and row volume snapshots.

## Optional Supabase CLI Inspect Commands

If the Supabase CLI is installed and authenticated for the project, these commands can provide supporting evidence. They are optional and must not replace the SQL baseline above.

```powershell
supabase inspect db role-connections
supabase inspect db long-running-queries
supabase inspect db outliers
supabase inspect db cache-hit
supabase inspect db blocking
```

Record exit codes and sanitized summaries only. Do not paste raw query text or user data from CLI output into evidence.

## Capture Template for Results and Interpretation

```markdown
## Capture Metadata

- Captured at UTC:
- Captured by:
- Environment: Supabase/Postgres
- Connection mode: direct or pooler, if known without revealing URL
- `DATABASE_URL` present: yes/no, value not recorded
- `psql` available: yes/no

## Command Outcomes

| Check | Command | Exit code | Sanitized summary |
| --- | --- | --- | --- |
| Extension | `psql "$env:DATABASE_URL" -v ON_ERROR_STOP=1 -c "SELECT extname FROM pg_extension WHERE extname = 'pg_stat_statements';"` | | |
| Total time | top queries by total execution time SQL | | |
| Call volume | top queries by call volume SQL | | |
| Row volume | top queries by row volume SQL | | |
| Connections | `pg_stat_activity` snapshot SQL | | |
| Cache hit | cache hit baseline SQL | | |

## Sanitized Baseline Findings

- Top total-time `queryid` values:
- Top call-volume `queryid` values:
- Top row-volume `queryid` values:
- Connection snapshot:
- Cache hit baseline:
- Immediate interpretation for Tasks 2-3:
- Follow-up checks requiring DB credentials or elevated stats permissions:
```

## Optional Troubleshooting: Raw Query Text

Raw query text is not part of the primary baseline. If aggregate metrics are insufficient to map a `queryid` to an in-repo code path, an operator may run a tightly scoped troubleshooting query and sanitize the result before sharing it.

```sql
SELECT
  queryid,
  left(query, 120) AS redacted_query_sample
FROM pg_stat_statements
WHERE dbid = (SELECT oid FROM pg_database WHERE datname = current_database())
  AND queryid = $TARGET_QUERY_ID
LIMIT 1;
```

Before recording any sample, remove literals, identifiers that expose tenant/user details, and any business-sensitive values. Prefer codebase search by SQL shape over saving query text.
