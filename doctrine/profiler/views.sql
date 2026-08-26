-- ============================================================================
-- profiler/views.sql — Trireme doctrine: the yardsticks (m01–m13) + audit views
-- ============================================================================
-- Status: EXECUTABLE BOOKKEEPING LAW (Q11d). Any change to a view here is a
-- [yardstick] change — flag the commit and the CHANGELOG entry.
-- Machinery (ingest, report generator, glance pane) lives OUTSIDE doctrine and
-- treats this file as read-only law. The profiler OBSERVES, never enforces.
--
-- Runs against the per-sortie SQLite read-model (separate file from the board
-- DB), built by ingest machinery at sortie close from the sortie JSONL.
-- Degraded mode: model.* / tool.* events absent -> those views return no rows.
-- Every metric view carries doctrine_hash as a first-class dimension.
-- Synthetic events (Q8 injected failures) are EXCLUDED by default via v_events;
-- query the events table directly to include them.
-- ============================================================================

-- ---------------------------------------------------------------------------
-- Canonical event store (DDL is law; see CONTRACTS/ENVELOPE.md)
-- ---------------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS events (
  ts            TEXT    NOT NULL,           -- ISO-8601 UTC
  sortie_id     TEXT    NOT NULL,
  doctrine_hash TEXT    NOT NULL,
  seat          TEXT    NOT NULL,
  task_id       TEXT,                        -- NULL for sortie-scoped events
  type          TEXT    NOT NULL,
  synthetic     INTEGER NOT NULL DEFAULT 0,
  payload       TEXT    NOT NULL DEFAULT '{}' -- JSON
);
CREATE INDEX IF NOT EXISTS idx_events_task ON events(task_id, type);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type, doctrine_hash);

-- Reconciliation source tables (double-entry partners; populated by ingest
-- machinery from 5a board rows, 5c main commits, and provider usage APIs).
CREATE TABLE IF NOT EXISTS recon_board_tasks (
  task_id TEXT PRIMARY KEY, status TEXT NOT NULL, sortie_id TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recon_git_main (
  commit_hash TEXT PRIMARY KEY, task_id TEXT, sortie_id TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS recon_provider_usage (
  sortie_id TEXT NOT NULL, provider TEXT NOT NULL,
  tokens_in INTEGER NOT NULL, tokens_out INTEGER NOT NULL, cost_usd REAL NOT NULL);

-- ---------------------------------------------------------------------------
-- Base view: synthetic excluded by default (Q11b)
-- ---------------------------------------------------------------------------
DROP VIEW IF EXISTS v_events;
CREATE VIEW v_events AS
SELECT * FROM events WHERE synthetic = 0;

-- ===========================================================================
-- FLEET METRICS (m01–m06) — judge orchestration quality (layer 3)
-- ===========================================================================

-- m01 — cycle time by LIFECYCLE STAGE (queue/build/verify/merge — "stage",
-- not project phase; see m01b for that). Decision fed: where task wall-time
-- goes; decomposition sizing. Tension partners: m02, m11 (confetti
-- decomposition shrinks cycle time but blows first-pass volume and calls/task).
DROP VIEW IF EXISTS m01_cycle_time_by_stage;
CREATE VIEW m01_cycle_time_by_stage AS
WITH marks AS (
  SELECT task_id, sortie_id, doctrine_hash,
    MIN(CASE WHEN type = 'task.created'   THEN ts END) AS created,
    MIN(CASE WHEN type = 'task.claimed'   THEN ts END) AS claimed,
    MAX(CASE WHEN type = 'task.completed' THEN ts END) AS last_completed,
    MIN(CASE WHEN type = 'verdict.pass'   THEN ts END) AS passed,
    MIN(CASE WHEN type = 'task.merged'    THEN ts END) AS merged
  FROM v_events WHERE task_id IS NOT NULL
  GROUP BY task_id, sortie_id, doctrine_hash
)
SELECT task_id, sortie_id, doctrine_hash,
  ROUND((julianday(claimed)        - julianday(created))        * 86400.0, 1) AS queue_s,
  ROUND((julianday(last_completed) - julianday(claimed))        * 86400.0, 1) AS build_s,
  ROUND((julianday(passed)         - julianday(last_completed)) * 86400.0, 1) AS verify_s,
  ROUND((julianday(merged)         - julianday(passed))         * 86400.0, 1) AS merge_s,
  ROUND((julianday(merged)         - julianday(created))        * 86400.0, 1) AS total_s
FROM marks;

-- m01b — cycle time by PROJECT PHASE (the Phase-1/Phase-2 axis; redline round
-- 1: "let's have both"). Decision fed: is the fleet speeding up or slowing
-- down as the project matures? Rolls m01's per-task stages up per phase.
DROP VIEW IF EXISTS m01b_cycle_time_by_project_phase;
CREATE VIEW m01b_cycle_time_by_project_phase AS
WITH phased AS (
  SELECT s.*, json_extract(c.payload, '$.phase') AS project_phase
  FROM m01_cycle_time_by_stage s
  JOIN v_events c ON c.type = 'task.created' AND c.task_id = s.task_id
                 AND c.sortie_id = s.sortie_id
  WHERE s.total_s IS NOT NULL  -- completed lifecycles only; in-flight tasks
                               -- would inflate the count under NULL averages
)
SELECT project_phase, sortie_id, doctrine_hash, COUNT(*) AS tasks,
  ROUND(AVG(queue_s),  1) AS avg_queue_s,
  ROUND(AVG(build_s),  1) AS avg_build_s,
  ROUND(AVG(verify_s), 1) AS avg_verify_s,
  ROUND(AVG(merge_s),  1) AS avg_merge_s,
  ROUND(AVG(total_s),  1) AS avg_total_s,
  MAX(total_s)            AS max_total_s
FROM phased GROUP BY project_phase, sortie_id, doctrine_hash;

-- m02 — first-pass yield. Decision fed: is decomposition producing tasks
-- workers get right the first time? Tension partners: m01, m07 (yield bought
-- with tiny tasks shows up as cycle-time/token overhead).
DROP VIEW IF EXISTS m02_first_pass_yield;
CREATE VIEW m02_first_pass_yield AS
WITH merged AS (
  SELECT DISTINCT task_id, sortie_id, doctrine_hash
  FROM v_events WHERE type = 'task.merged'
),
rejected AS (
  SELECT DISTINCT task_id FROM v_events WHERE type = 'task.rejected'
)
SELECT m.sortie_id, m.doctrine_hash,
  COUNT(*)                                            AS merged_tasks,
  SUM(CASE WHEN r.task_id IS NULL THEN 1 ELSE 0 END)  AS first_pass_tasks,
  ROUND(1.0 * SUM(CASE WHEN r.task_id IS NULL THEN 1 ELSE 0 END) / COUNT(*), 3)
                                                      AS first_pass_yield
FROM merged m LEFT JOIN rejected r ON r.task_id = m.task_id
GROUP BY m.sortie_id, m.doctrine_hash;

-- m03 — rejection-attribution split. Decision fed: is failure landing on
-- workers (implementation) or the Producer (decomposition)? First-class
-- attribution dimension. Includes budget blowouts, merge-gate failures and
-- mechanical scope violations. seat.error is deliberately ABSENT: infra
-- failures are nobody's work quality (they get m14).
DROP VIEW IF EXISTS m03_rejection_attribution;
CREATE VIEW m03_rejection_attribution AS
SELECT sortie_id, doctrine_hash,
  json_extract(payload, '$.charged') AS charged,
  type                               AS cause,
  COUNT(*)                           AS n
FROM v_events
WHERE type IN ('task.rejected', 'task.merge_conflict',
               'task.suite_failed_post_rebase', 'budget.cap_hit',
               'task.scope_violation')
GROUP BY sortie_id, doctrine_hash, charged, cause;

-- m04 — redone-work rate. Decision fed: how much fleet effort is re-work?
-- (the Principal's original requirement: "lack of redone work".)
DROP VIEW IF EXISTS m04_redone_work_rate;
CREATE VIEW m04_redone_work_rate AS
WITH attempts AS (
  SELECT task_id, sortie_id, doctrine_hash, COUNT(*) AS n
  FROM v_events WHERE type = 'task.completed'
  GROUP BY task_id, sortie_id, doctrine_hash
)
SELECT sortie_id, doctrine_hash,
  COUNT(*)                                   AS attempted_tasks,
  SUM(CASE WHEN n > 1 THEN 1 ELSE 0 END)     AS redone_tasks,
  SUM(n) - COUNT(*)                          AS extra_attempts,
  ROUND(1.0 * SUM(CASE WHEN n > 1 THEN 1 ELSE 0 END) / COUNT(*), 3)
                                             AS redone_rate
FROM attempts GROUP BY sortie_id, doctrine_hash;

-- m05 — human touches per sortie. Decision fed: doctrine holes. The happy
-- path has zero human touches (Q8 exit criterion); every touch is a hole and
-- files a ruling at sortie close.
DROP VIEW IF EXISTS m05_human_touches_per_sortie;
CREATE VIEW m05_human_touches_per_sortie AS
SELECT sortie_id, doctrine_hash, COUNT(*) AS touches,
  json_group_array(json_extract(payload, '$.reason')) AS reasons
FROM v_events WHERE type = 'human.touch'
GROUP BY sortie_id, doctrine_hash;

-- m06 — CLARIFY rate. Decision fed: spec completeness at decomposition time.
-- A loud-gap detector by construction (unspeccable goes up).
DROP VIEW IF EXISTS m06_clarify_rate;
CREATE VIEW m06_clarify_rate AS
WITH t AS (
  SELECT sortie_id, doctrine_hash, COUNT(*) AS tasks
  FROM v_events WHERE type = 'task.created'
  GROUP BY sortie_id, doctrine_hash
),
c AS (
  SELECT sortie_id, doctrine_hash, COUNT(*) AS clarifies
  FROM v_events WHERE type = 'task.clarify'
  GROUP BY sortie_id, doctrine_hash
)
SELECT t.sortie_id, t.doctrine_hash, t.tasks,
  COALESCE(c.clarifies, 0)                            AS clarifies,
  ROUND(1.0 * COALESCE(c.clarifies, 0) / t.tasks, 3)  AS clarify_rate
FROM t LEFT JOIN c ON c.sortie_id = t.sortie_id AND c.doctrine_hash = t.doctrine_hash;

-- ===========================================================================
-- HARNESS METRICS (m07–m12) — judge seat performance (4) and assignment
-- economics (7b). Empty in degraded mode.
-- ===========================================================================

-- m07 — tokens per merged task, by seat + model. Decision fed: Q9's empirical
-- question — do cheaper workers' rejection rates erase their savings?
DROP VIEW IF EXISTS m07_tokens_per_merged_task;
CREATE VIEW m07_tokens_per_merged_task AS
WITH spend AS (
  SELECT sortie_id, doctrine_hash, seat,
    json_extract(payload, '$.model') AS model,
    SUM(CAST(json_extract(payload, '$.tokens_in')  AS INTEGER)
      + CAST(json_extract(payload, '$.tokens_out') AS INTEGER)) AS tokens
  FROM v_events WHERE type = 'model.call'
  GROUP BY sortie_id, doctrine_hash, seat, model
),
merged AS (
  SELECT sortie_id, doctrine_hash, COUNT(DISTINCT task_id) AS merged_tasks
  FROM v_events WHERE type = 'task.merged'
  GROUP BY sortie_id, doctrine_hash
)
SELECT s.sortie_id, s.doctrine_hash, s.seat, s.model, s.tokens,
  m.merged_tasks,
  ROUND(1.0 * s.tokens / NULLIF(m.merged_tasks, 0), 1) AS tokens_per_merged_task
FROM spend s
JOIN merged m ON m.sortie_id = s.sortie_id AND m.doctrine_hash = s.doctrine_hash;

-- m08 — cost per merged task. Decision fed: assignment economics headline.
-- Tension partner: m02 (cheap fleet with low yield is not cheap).
DROP VIEW IF EXISTS m08_cost_per_merged_task;
CREATE VIEW m08_cost_per_merged_task AS
WITH spend AS (
  SELECT sortie_id, doctrine_hash,
    ROUND(SUM(CAST(json_extract(payload, '$.cost_usd') AS REAL)), 4) AS cost_usd
  FROM v_events WHERE type = 'model.call'
  GROUP BY sortie_id, doctrine_hash
),
merged AS (
  SELECT sortie_id, doctrine_hash, COUNT(DISTINCT task_id) AS merged_tasks
  FROM v_events WHERE type = 'task.merged'
  GROUP BY sortie_id, doctrine_hash
)
SELECT s.sortie_id, s.doctrine_hash, s.cost_usd, m.merged_tasks,
  ROUND(s.cost_usd / NULLIF(m.merged_tasks, 0), 4) AS cost_per_merged_task
FROM spend s
JOIN merged m ON m.sortie_id = s.sortie_id AND m.doctrine_hash = s.doctrine_hash;

-- m09 — model call latency p50/p95. Decision fed: provider/model swap timing;
-- sortie wall-time diagnosis.
DROP VIEW IF EXISTS m09_call_latency;
CREATE VIEW m09_call_latency AS
WITH calls AS (
  SELECT sortie_id, doctrine_hash,
    json_extract(payload, '$.model') AS model,
    CAST(json_extract(payload, '$.latency_ms') AS REAL) AS latency_ms
  FROM v_events WHERE type = 'model.call'
),
ranked AS (
  SELECT *, PERCENT_RANK() OVER (
    PARTITION BY sortie_id, doctrine_hash, model ORDER BY latency_ms) AS pr
  FROM calls
)
SELECT sortie_id, doctrine_hash, model, COUNT(*) AS calls,
  MAX(CASE WHEN pr <= 0.50 THEN latency_ms END) AS p50_ms,
  MAX(CASE WHEN pr <= 0.95 THEN latency_ms END) AS p95_ms
FROM ranked GROUP BY sortie_id, doctrine_hash, model;

-- m10 — tool failure rate. Decision fed: harness reliability; a flaky
-- test-runner poisons verdict evidence upstream of any model question.
DROP VIEW IF EXISTS m10_tool_failure_rate;
CREATE VIEW m10_tool_failure_rate AS
SELECT sortie_id, doctrine_hash,
  json_extract(payload, '$.tool') AS tool,
  COUNT(*)                        AS calls,
  SUM(CASE WHEN json_extract(payload, '$.ok') IN (0, 'false') THEN 1 ELSE 0 END)
                                  AS failures,
  ROUND(1.0 * SUM(CASE WHEN json_extract(payload, '$.ok') IN (0, 'false')
                       THEN 1 ELSE 0 END) / COUNT(*), 3) AS failure_rate
FROM v_events WHERE type = 'tool.call'
GROUP BY sortie_id, doctrine_hash, tool;

-- m11 — calls per task (model + tool). Decision fed: task-size sanity;
-- tension partner of m01/m02 against confetti decomposition.
DROP VIEW IF EXISTS m11_calls_per_task;
CREATE VIEW m11_calls_per_task AS
WITH per_task AS (
  SELECT sortie_id, doctrine_hash, task_id,
    SUM(CASE WHEN type = 'model.call' THEN 1 ELSE 0 END) AS model_calls,
    SUM(CASE WHEN type = 'tool.call'  THEN 1 ELSE 0 END) AS tool_calls
  FROM v_events
  WHERE task_id IS NOT NULL AND type IN ('model.call', 'tool.call')
  GROUP BY sortie_id, doctrine_hash, task_id
)
SELECT sortie_id, doctrine_hash, COUNT(*) AS tasks,
  ROUND(AVG(model_calls), 1) AS avg_model_calls,
  ROUND(AVG(tool_calls), 1)  AS avg_tool_calls,
  MAX(model_calls)           AS max_model_calls
FROM per_task GROUP BY sortie_id, doctrine_hash;

-- m12 — budget-cap hits. Decision fed: Producer estimation quality (blowouts
-- charge the Producer, Q4) and cap tuning (caps are config, not constants).
DROP VIEW IF EXISTS m12_budget_cap_hits;
CREATE VIEW m12_budget_cap_hits AS
SELECT sortie_id, doctrine_hash, seat,
  json_extract(payload, '$.kind') AS kind,
  COUNT(*)                        AS hits
FROM v_events WHERE type = 'budget.cap_hit'
GROUP BY sortie_id, doctrine_hash, seat, kind;

-- m14 — seat infrastructure failures. Decision fed: provider/harness
-- reliability, kept OUT of the blame views (m03) by design — an api_error
-- is not a rejection. Rising m14 with flat m03 = infrastructure problem;
-- rising together = look harder.
DROP VIEW IF EXISTS m14_seat_errors;
CREATE VIEW m14_seat_errors AS
SELECT sortie_id, doctrine_hash,
  json_extract(payload, '$.seat')   AS failed_seat,
  json_extract(payload, '$.reason') AS reason,
  COUNT(*)                          AS n
FROM v_events WHERE type = 'seat.error'
GROUP BY sortie_id, doctrine_hash, failed_seat, reason;

-- m13 — remediation ledger (redline round 1: wrong-approach merges are fixed
-- forward by follow-up tasks, and "those fixes should be clearly tracked in
-- reporting"). Decision fed: is the PASS-against-contract-as-written policy
-- (ROLES/VERIFIER.md) leaking bad approaches into main faster than they get
-- fixed? Tension partner: m02 (first-pass yield looks clean while remediation
-- debt grows). One row per spec-concern PASS with its remediation status,
-- plus remediation tasks whose origin never filed a concern (found later).
DROP VIEW IF EXISTS m13_remediation;
CREATE VIEW m13_remediation AS
WITH concerns AS (
  SELECT DISTINCT task_id, sortie_id, doctrine_hash
  FROM v_events
  WHERE type = 'verdict.pass'
    AND CAST(json_extract(payload, '$.spec_concern') AS INTEGER) = 1
),
remediations AS (
  SELECT task_id, sortie_id, doctrine_hash,
    json_extract(payload, '$.remediation_of') AS remediation_of
  FROM v_events
  WHERE type = 'task.created'
    AND json_extract(payload, '$.remediation_of') IS NOT NULL
),
merged AS (SELECT DISTINCT task_id FROM v_events WHERE type = 'task.merged')
SELECT
  c.sortie_id, c.doctrine_hash,
  c.task_id                                            AS concern_task,
  r.task_id                                            AS remediation_task,
  CASE WHEN r.task_id IS NULL THEN 0 ELSE 1 END        AS remediated,
  CASE WHEN m.task_id IS NULL THEN 0 ELSE 1 END        AS remediation_merged
FROM concerns c
LEFT JOIN remediations r ON r.remediation_of = c.task_id
LEFT JOIN merged m       ON m.task_id = r.task_id
UNION ALL
-- remediation without a recorded concern: the approach problem was found
-- after the verdict (still remediation, still visible)
SELECT r.sortie_id, r.doctrine_hash,
  r.remediation_of, r.task_id, 1,
  CASE WHEN m.task_id IS NULL THEN 0 ELSE 1 END
FROM remediations r
LEFT JOIN concerns c ON c.task_id = r.remediation_of
LEFT JOIN merged m   ON m.task_id = r.task_id
WHERE c.task_id IS NULL;
-- (FULL OUTER JOIN avoided deliberately — SQLite >= 3.37 floor, same as a03.)

-- ===========================================================================
-- AUDIT VIEWS — double-entry reconciliation (Q11e instrument integrity).
-- Journal (events) vs. board (5a), git main (5c), provider usage APIs.
-- Any row returned = daylight = FLASH + quarantine (enforced by 3b, not here —
-- the profiler observes, never enforces). Audits INCLUDE synthetic events:
-- injected failures still produce board rows and must reconcile.
-- ===========================================================================

-- a01 — journal vs. board: merged-in-journal must be done-on-board, and back.
DROP VIEW IF EXISTS a01_journal_vs_board;
CREATE VIEW a01_journal_vs_board AS
SELECT 'merged_in_journal_not_done_on_board' AS daylight, j.task_id, j.sortie_id
FROM (SELECT DISTINCT task_id, sortie_id FROM events WHERE type = 'task.merged') j
LEFT JOIN recon_board_tasks b ON b.task_id = j.task_id AND b.status = 'done'
WHERE b.task_id IS NULL
UNION ALL
SELECT 'done_on_board_not_merged_in_journal', b.task_id, b.sortie_id
FROM recon_board_tasks b
LEFT JOIN (SELECT DISTINCT task_id FROM events WHERE type = 'task.merged') j
  ON j.task_id = b.task_id
WHERE b.status = 'done' AND j.task_id IS NULL;

-- a02 — journal vs. git main: every task.merged commit exists on main, and
-- every main commit traces to a task.merged event (always-green trunk law).
DROP VIEW IF EXISTS a02_journal_vs_git;
CREATE VIEW a02_journal_vs_git AS
SELECT 'merge_event_commit_not_on_main' AS daylight,
  j.task_id, j.sortie_id, json_extract(j.payload, '$.commit') AS commit_hash
FROM events j
LEFT JOIN recon_git_main g ON g.commit_hash = json_extract(j.payload, '$.commit')
WHERE j.type = 'task.merged' AND g.commit_hash IS NULL
UNION ALL
SELECT 'main_commit_without_merge_event', g.task_id, g.sortie_id, g.commit_hash
FROM recon_git_main g
LEFT JOIN events j
  ON j.type = 'task.merged' AND json_extract(j.payload, '$.commit') = g.commit_hash
WHERE j.ts IS NULL;

-- a03 — journal vs. provider usage: 6d-emitted spend vs. the provider's own
-- ledger. Deltas reported per sortie+provider; the FLASH threshold
-- (reconciliation_tolerance_pct) is config and applied by 3b.
DROP VIEW IF EXISTS a03_journal_vs_provider;
CREATE VIEW a03_journal_vs_provider AS
WITH journal AS (
  SELECT sortie_id, json_extract(payload, '$.provider') AS provider,
    SUM(CAST(json_extract(payload, '$.tokens_in')  AS INTEGER)) AS j_tokens_in,
    SUM(CAST(json_extract(payload, '$.tokens_out') AS INTEGER)) AS j_tokens_out,
    ROUND(SUM(CAST(json_extract(payload, '$.cost_usd') AS REAL)), 4) AS j_cost_usd
  FROM events WHERE type = 'model.call'
  GROUP BY sortie_id, provider
)
SELECT
  j.sortie_id, j.provider,
  j.j_tokens_in,  p.tokens_in  AS provider_tokens_in,
  j.j_tokens_out, p.tokens_out AS provider_tokens_out,
  j.j_cost_usd,   p.cost_usd   AS provider_cost_usd,
  ROUND(COALESCE(j.j_cost_usd, 0) - COALESCE(p.cost_usd, 0), 4) AS cost_delta_usd
FROM journal j
LEFT JOIN recon_provider_usage p
  ON p.sortie_id = j.sortie_id AND p.provider = j.provider
UNION ALL
-- provider ledger rows with no journal counterpart (daylight by absence)
SELECT p.sortie_id, p.provider,
  NULL, p.tokens_in, NULL, p.tokens_out, NULL, p.cost_usd,
  ROUND(0 - p.cost_usd, 4)
FROM recon_provider_usage p
LEFT JOIN journal j ON j.sortie_id = p.sortie_id AND j.provider = p.provider
WHERE j.provider IS NULL;
-- (FULL OUTER JOIN avoided deliberately: doctrine law must execute on SQLite
--  >= 3.37 — the degraded-mode floor on the current box.)
