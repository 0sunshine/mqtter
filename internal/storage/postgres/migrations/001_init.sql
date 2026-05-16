CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS schema_migrations (
  version text PRIMARY KEY,
  applied_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS device_types (
  code text PRIMARY KEY,
  name text NOT NULL,
  description text NOT NULL DEFAULT '',
  schema_json text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now()
);

INSERT INTO device_types (code, name, description)
VALUES ('unknown', 'Unknown Device', 'Default type for newly connected devices')
ON CONFLICT (code) DO NOTHING;

CREATE TABLE IF NOT EXISTS devices (
  id text PRIMARY KEY,
  client_id text NOT NULL UNIQUE,
  type text NOT NULL DEFAULT 'unknown' REFERENCES device_types(code),
  status text NOT NULL,
  session_id text NOT NULL DEFAULT '',
  first_seen_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  last_disconnected_at timestamptz,
  metadata jsonb NOT NULL DEFAULT '{}'::jsonb
);

CREATE INDEX IF NOT EXISTS idx_devices_status ON devices(status);
CREATE INDEX IF NOT EXISTS idx_devices_type ON devices(type);
CREATE INDEX IF NOT EXISTS idx_devices_last_seen ON devices(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS observed_topics (
  id bigserial PRIMARY KEY,
  device_id text REFERENCES devices(id) ON DELETE SET NULL,
  client_id text NOT NULL,
  topic text NOT NULL,
  kind text NOT NULL,
  qos smallint NOT NULL DEFAULT 0,
  first_seen_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL,
  UNIQUE (client_id, topic, kind)
);

CREATE INDEX IF NOT EXISTS idx_observed_topics_device ON observed_topics(device_id);
CREATE INDEX IF NOT EXISTS idx_observed_topics_topic ON observed_topics(topic);
CREATE INDEX IF NOT EXISTS idx_observed_topics_last_seen ON observed_topics(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS mqtt_messages (
  id text NOT NULL,
  device_id text REFERENCES devices(id) ON DELETE SET NULL,
  client_id text NOT NULL,
  session_id text NOT NULL DEFAULT '',
  topic text NOT NULL,
  payload_text text NOT NULL,
  payload_format text NOT NULL,
  qos smallint NOT NULL,
  retain boolean NOT NULL,
  received_at timestamptz NOT NULL,
  PRIMARY KEY (id, received_at)
) PARTITION BY RANGE (received_at);

CREATE TABLE IF NOT EXISTS mqtt_messages_default PARTITION OF mqtt_messages DEFAULT;
CREATE INDEX IF NOT EXISTS idx_mqtt_messages_default_topic_time ON mqtt_messages_default(topic, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_mqtt_messages_default_device_time ON mqtt_messages_default(device_id, received_at DESC);

CREATE TABLE IF NOT EXISTS publish_commands (
  id text PRIMARY KEY,
  admin_user_id text NOT NULL,
  topic text NOT NULL,
  payload_text text NOT NULL,
  qos smallint NOT NULL,
  retain boolean NOT NULL,
  status text NOT NULL,
  error text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL,
  published_at timestamptz
);

CREATE INDEX IF NOT EXISTS idx_publish_commands_created ON publish_commands(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_publish_commands_topic ON publish_commands(topic);

CREATE TABLE IF NOT EXISTS system_alerts (
  id text PRIMARY KEY,
  level text NOT NULL,
  code text NOT NULL UNIQUE,
  message text NOT NULL,
  status text NOT NULL,
  first_seen_at timestamptz NOT NULL,
  last_seen_at timestamptz NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_system_alerts_status ON system_alerts(status);
CREATE INDEX IF NOT EXISTS idx_system_alerts_last_seen ON system_alerts(last_seen_at DESC);

CREATE TABLE IF NOT EXISTS admin_users (
  id text PRIMARY KEY,
  username text NOT NULL UNIQUE,
  password_hash bytea NOT NULL,
  role text NOT NULL,
  disabled boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS admin_sessions (
  token text PRIMARY KEY,
  user_id text NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  expires_at timestamptz NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_admin_sessions_expires ON admin_sessions(expires_at);

CREATE TABLE IF NOT EXISTS device_type_changes (
  id bigserial PRIMARY KEY,
  device_id text NOT NULL REFERENCES devices(id) ON DELETE CASCADE,
  old_type text NOT NULL,
  new_type text NOT NULL,
  actor_id text NOT NULL,
  changed_at timestamptz NOT NULL
);
