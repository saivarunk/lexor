# ADR 002 — SQLite for private user state

Status: accepted.

Use modernc.org/sqlite to keep builds CGO-free. Embed numbered SQL migrations and record each application transactionally. Enable WAL, busy timeout and foreign keys, with one local connection.

SQLite gives atomic progress/activity updates, durable drafts/history and low operational overhead. Executions retain JSON results and source; no external database/ORM is needed. Calendar activity uses the workspace's initial timezone, persisted independently of later travel.
