CREATE TABLE IF NOT EXISTS quick_actions (
  id text PRIMARY KEY,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  admin_user_id text NOT NULL REFERENCES admin_users(id) ON DELETE RESTRICT,
  name text NOT NULL,
  topic text NOT NULL,
  payload_text text NOT NULL,
  qos smallint NOT NULL,
  retain boolean NOT NULL,
  created_at timestamptz NOT NULL,
  updated_at timestamptz NOT NULL,
  UNIQUE (device_id, name)
);

CREATE INDEX IF NOT EXISTS idx_quick_actions_device ON quick_actions(device_id, created_at DESC);
