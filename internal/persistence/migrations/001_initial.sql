CREATE TABLE code_drafts (problem_id TEXT NOT NULL, language TEXT NOT NULL, source TEXT NOT NULL, updated_at TEXT NOT NULL, PRIMARY KEY(problem_id,language));
CREATE TABLE executions (id TEXT PRIMARY KEY, problem_id TEXT NOT NULL, language TEXT NOT NULL, source TEXT NOT NULL, mode TEXT NOT NULL CHECK(mode IN ('run','submit')), status TEXT NOT NULL, created_at TEXT NOT NULL, result_json TEXT NOT NULL);
CREATE INDEX executions_problem_created ON executions(problem_id,created_at DESC);
CREATE INDEX executions_mode_created ON executions(mode,created_at DESC);
CREATE TABLE problem_progress (problem_id TEXT PRIMARY KEY, status TEXT NOT NULL, attempts INTEGER NOT NULL, best_duration_ms REAL NOT NULL DEFAULT 0, first_attempted_at TEXT NOT NULL, last_attempted_at TEXT NOT NULL, first_solved_at TEXT, last_solved_at TEXT);
CREATE TABLE daily_activity (date TEXT PRIMARY KEY, count INTEGER NOT NULL DEFAULT 0, new_solves INTEGER NOT NULL DEFAULT 0);
CREATE TABLE settings (key TEXT PRIMARY KEY, value TEXT NOT NULL);
CREATE TABLE registry_installations (id INTEGER PRIMARY KEY, source TEXT NOT NULL, imported_at TEXT NOT NULL, problem_ids TEXT NOT NULL);
