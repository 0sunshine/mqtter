CREATE TABLE IF NOT EXISTS scheduled_publish_tasks (
  id text PRIMARY KEY,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  admin_user_id text NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
  name text NOT NULL DEFAULT '',
  topic text NOT NULL,
  payload_text text NOT NULL,
  qos smallint NOT NULL,
  retain boolean NOT NULL,
  schedule_type text NOT NULL,
  run_at timestamptz,
  time_of_day text NOT NULL DEFAULT '',
  weekdays text NOT NULL DEFAULT '',
  timezone text NOT NULL DEFAULT 'Asia/Hong_Kong',
  status text NOT NULL,
  next_run_at timestamptz,
  last_run_at timestamptz,
  last_error text NOT NULL DEFAULT '',
  run_count integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduled_publish_tasks_device ON scheduled_publish_tasks(device_id);
CREATE INDEX IF NOT EXISTS idx_scheduled_publish_tasks_due ON scheduled_publish_tasks(status, next_run_at);

CREATE TABLE IF NOT EXISTS scheduled_publish_runs (
  id text PRIMARY KEY,
  task_id text NOT NULL REFERENCES scheduled_publish_tasks(id) ON DELETE CASCADE,
  publish_command_id text REFERENCES publish_commands(id) ON DELETE SET NULL,
  status text NOT NULL,
  error text NOT NULL DEFAULT '',
  scheduled_for timestamptz NOT NULL,
  started_at timestamptz NOT NULL,
  finished_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_scheduled_publish_runs_task ON scheduled_publish_runs(task_id, started_at DESC);
