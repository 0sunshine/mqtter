package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"mqtter/internal/domain"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database url is required")
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &Store{pool: pool}, nil
}

func (s *Store) Close() {
	if s != nil && s.pool != nil {
		s.pool.Close()
	}
}

func (s *Store) UpsertConnected(ctx context.Context, ev domain.ConnectEvent) (domain.DeviceDTO, error) {
	row := s.pool.QueryRow(ctx, `
INSERT INTO devices (id, client_id, type, status, session_id, first_seen_at, last_seen_at, metadata)
VALUES (gen_random_uuid()::text, $1, 'unknown', 'online', $2, $3, $3,
        jsonb_build_object('username', $4::text, 'remote', $5::text, 'listener', $6::text, 'protocolVersion', $7::int))
ON CONFLICT (client_id) DO UPDATE
SET status='online',
    session_id=EXCLUDED.session_id,
    last_seen_at=EXCLUDED.last_seen_at,
    metadata=devices.metadata || EXCLUDED.metadata
RETURNING id, client_id, type, status, session_id, first_seen_at, last_seen_at, last_disconnected_at, metadata`,
		ev.ClientID, ev.SessionID, ev.ConnectedAt, ev.Username, ev.Remote, ev.Listener, int(ev.ProtocolVersion))
	return scanDevice(row)
}

func (s *Store) RecordSubscription(ctx context.Context, ev domain.SubscribeEvent) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	deviceID, err := ensureDevice(ctx, tx, ev.ClientID, ev.SessionID, ev.ObservedAt)
	if err != nil {
		return err
	}
	for _, sub := range ev.Subscriptions {
		if _, err := tx.Exec(ctx, `
INSERT INTO observed_topics (device_id, client_id, topic, kind, qos, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (client_id, topic, kind) DO UPDATE
SET device_id=EXCLUDED.device_id, qos=EXCLUDED.qos, last_seen_at=EXCLUDED.last_seen_at`,
			deviceID, ev.ClientID, sub.Filter, domain.TopicKindSubscribeFilter, int(sub.QoS), ev.ObservedAt); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) PersistPublish(ctx context.Context, ev domain.PublishEvent) (domain.MessageDTO, error) {
	if err := s.EnsureMessagePartition(ctx, ev.ReceivedAt); err != nil {
		return domain.MessageDTO{}, err
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.MessageDTO{}, err
	}
	defer tx.Rollback(ctx)

	var deviceID *string
	kind := domain.TopicKindPublishTopic
	if ev.ClientID == "inline" {
		kind = domain.TopicKindAdminPublishTopic
	} else {
		id, err := ensureDevice(ctx, tx, ev.ClientID, ev.SessionID, ev.ReceivedAt)
		if err != nil {
			return domain.MessageDTO{}, err
		}
		deviceID = &id
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO observed_topics (device_id, client_id, topic, kind, qos, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $6)
ON CONFLICT (client_id, topic, kind) DO UPDATE
SET device_id=EXCLUDED.device_id, qos=EXCLUDED.qos, last_seen_at=EXCLUDED.last_seen_at`,
		deviceID, ev.ClientID, ev.Topic, kind, int(ev.QoS), ev.ReceivedAt); err != nil {
		return domain.MessageDTO{}, err
	}

	row := tx.QueryRow(ctx, `
INSERT INTO mqtt_messages (id, device_id, client_id, session_id, topic, payload_text, payload_format, qos, retain, received_at)
VALUES (gen_random_uuid()::text, $1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING id, device_id, client_id, session_id, topic, payload_text, payload_format, qos, retain, received_at`,
		deviceID, ev.ClientID, ev.SessionID, ev.Topic, ev.PayloadText, ev.PayloadFormat, int(ev.QoS), ev.Retain, ev.ReceivedAt)
	msg, err := scanMessage(row)
	if err != nil {
		return domain.MessageDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.MessageDTO{}, err
	}
	return msg, nil
}

func (s *Store) MarkDisconnected(ctx context.Context, ev domain.DisconnectEvent) error {
	_, err := s.pool.Exec(ctx, `
UPDATE devices
SET status='offline', last_disconnected_at=$2, last_seen_at=$2
WHERE client_id=$1 AND ($3='' OR session_id=$3)`,
		ev.ClientID, ev.DisconnectedAt, ev.SessionID)
	return err
}

func (s *Store) MarkStaleOnline(ctx context.Context, at time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `
UPDATE devices
SET status='stale_offline', last_disconnected_at=$1
WHERE status='online'`,
		at)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func (s *Store) ListDevices(ctx context.Context, f domain.DeviceFilter) (domain.Page[domain.DeviceDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT id, client_id, type, status, session_id, first_seen_at, last_seen_at, last_disconnected_at, metadata, count(*) OVER()
FROM devices
WHERE ($1='' OR status=$1)
  AND ($2='' OR type=$2)
  AND ($3='' OR client_id ILIKE '%' || $3 || '%')
ORDER BY last_seen_at DESC
LIMIT $4 OFFSET $5`,
		f.Status, f.Type, f.Query, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.DeviceDTO]{}, err
	}
	defer rows.Close()

	items := []domain.DeviceDTO{}
	total := 0
	for rows.Next() {
		device, rowTotal, err := scanDeviceWithTotal(rows)
		if err != nil {
			return domain.Page[domain.DeviceDTO]{}, err
		}
		items = append(items, device)
		total = rowTotal
	}
	return domain.Page[domain.DeviceDTO]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func (s *Store) GetDevice(ctx context.Context, id string) (domain.DeviceDTO, error) {
	return scanDevice(s.pool.QueryRow(ctx, `
SELECT id, client_id, type, status, session_id, first_seen_at, last_seen_at, last_disconnected_at, metadata
FROM devices WHERE id=$1`, id))
}

func (s *Store) ChangeDeviceType(ctx context.Context, cmd domain.ChangeDeviceTypeCommand, changedAt time.Time) (domain.DeviceDTO, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return domain.DeviceDTO{}, err
	}
	defer tx.Rollback(ctx)
	var oldType string
	if err := tx.QueryRow(ctx, `SELECT type FROM devices WHERE id=$1 FOR UPDATE`, cmd.DeviceID).Scan(&oldType); err != nil {
		return domain.DeviceDTO{}, err
	}
	row := tx.QueryRow(ctx, `
UPDATE devices SET type=$2 WHERE id=$1
RETURNING id, client_id, type, status, session_id, first_seen_at, last_seen_at, last_disconnected_at, metadata`,
		cmd.DeviceID, cmd.Type)
	device, err := scanDevice(row)
	if err != nil {
		return domain.DeviceDTO{}, err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO device_type_changes (device_id, old_type, new_type, actor_id, changed_at)
VALUES ($1, $2, $3, $4, $5)`,
		cmd.DeviceID, oldType, cmd.Type, cmd.ActorID, changedAt); err != nil {
		return domain.DeviceDTO{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return domain.DeviceDTO{}, err
	}
	return device, nil
}

func (s *Store) ListDeviceTypes(ctx context.Context) ([]domain.DeviceTypeDTO, error) {
	rows, err := s.pool.Query(ctx, `SELECT code, name, description, schema_json, created_at FROM device_types ORDER BY code`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.DeviceTypeDTO
	for rows.Next() {
		var item domain.DeviceTypeDTO
		if err := rows.Scan(&item.Code, &item.Name, &item.Description, &item.Schema, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListDeviceTopics(ctx context.Context, deviceID string, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT device_id, client_id, topic, kind, qos, first_seen_at, last_seen_at, count(*) OVER()
FROM observed_topics
WHERE device_id=$1
  AND ($2='' OR kind=$2)
  AND ($3='' OR topic ILIKE '%' || $3 || '%')
ORDER BY last_seen_at DESC
LIMIT $4 OFFSET $5`,
		deviceID, f.Direction, f.Query, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.ObservedTopicDTO]{}, err
	}
	defer rows.Close()
	return scanTopicPage(rows, f.Page, f.PageSize)
}

func (s *Store) ListTopics(ctx context.Context, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT device_id, client_id, topic, kind, qos, first_seen_at, last_seen_at, count(*) OVER()
FROM observed_topics
WHERE ($1='' OR kind=$1)
  AND ($2='' OR topic ILIKE '%' || $2 || '%')
ORDER BY last_seen_at DESC
LIMIT $3 OFFSET $4`,
		f.Direction, f.Query, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.ObservedTopicDTO]{}, err
	}
	defer rows.Close()
	return scanTopicPage(rows, f.Page, f.PageSize)
}

func (s *Store) RecordAdminPublishTopic(ctx context.Context, clientID, topic string, qos byte, at time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO observed_topics (device_id, client_id, topic, kind, qos, first_seen_at, last_seen_at)
VALUES (NULL, $1, $2, $3, $4, $5, $5)
ON CONFLICT (client_id, topic, kind) DO UPDATE
SET qos=EXCLUDED.qos, last_seen_at=EXCLUDED.last_seen_at`,
		clientID, topic, domain.TopicKindAdminPublishTopic, int(qos), at)
	return err
}

func (s *Store) QueryMessages(ctx context.Context, f domain.MessageFilter) (domain.Page[domain.MessageDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT id, device_id, client_id, session_id, topic, payload_text, payload_format, qos, retain, received_at, count(*) OVER()
FROM mqtt_messages
WHERE ($1='' OR device_id=$1)
  AND ($2='' OR topic=$2)
  AND ($3::timestamptz IS NULL OR received_at >= $3)
  AND ($4::timestamptz IS NULL OR received_at <= $4)
ORDER BY received_at DESC
LIMIT $5 OFFSET $6`,
		f.DeviceID, f.Topic, f.From, f.To, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.MessageDTO]{}, err
	}
	defer rows.Close()

	items := []domain.MessageDTO{}
	total := 0
	for rows.Next() {
		msg, rowTotal, err := scanMessageWithTotal(rows)
		if err != nil {
			return domain.Page[domain.MessageDTO]{}, err
		}
		items = append(items, msg)
		total = rowTotal
	}
	return domain.Page[domain.MessageDTO]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func (s *Store) CreatePublishCommand(ctx context.Context, id string, cmd domain.PublishCommand, createdAt time.Time) (domain.PublishCommandDTO, error) {
	row := s.pool.QueryRow(ctx, `
INSERT INTO publish_commands (id, admin_user_id, topic, payload_text, qos, retain, status, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING id, admin_user_id, topic, payload_text, qos, retain, status, error, created_at, published_at`,
		id, cmd.AdminUserID, cmd.Topic, cmd.PayloadText, int(cmd.QoS), cmd.Retain, domain.PublishStatusPending, createdAt)
	return scanCommand(row)
}

func (s *Store) MarkPublishCommand(ctx context.Context, id string, status domain.PublishStatus, errText string, publishedAt *time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE publish_commands SET status=$2, error=$3, published_at=$4 WHERE id=$1`, id, status, errText, publishedAt)
	return err
}

func (s *Store) ListPublishCommands(ctx context.Context, f domain.CommandFilter) (domain.Page[domain.PublishCommandDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT id, admin_user_id, topic, payload_text, qos, retain, status, error, created_at, published_at, count(*) OVER()
FROM publish_commands
WHERE ($1='' OR topic=$1)
  AND ($2='' OR status=$2)
ORDER BY created_at DESC
LIMIT $3 OFFSET $4`,
		f.Topic, f.Status, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.PublishCommandDTO]{}, err
	}
	defer rows.Close()
	items := []domain.PublishCommandDTO{}
	total := 0
	for rows.Next() {
		item, rowTotal, err := scanCommandWithTotal(rows)
		if err != nil {
			return domain.Page[domain.PublishCommandDTO]{}, err
		}
		items = append(items, item)
		total = rowTotal
	}
	return domain.Page[domain.PublishCommandDTO]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func (s *Store) CreateScheduledTask(ctx context.Context, task domain.ScheduledPublishTaskDTO) (domain.ScheduledPublishTaskDTO, error) {
	row := s.pool.QueryRow(ctx, `
INSERT INTO scheduled_publish_tasks (
  id, device_id, admin_user_id, name, topic, payload_text, qos, retain,
  schedule_type, run_at, time_of_day, weekdays, timezone, status, next_run_at,
  created_at, updated_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $16)
RETURNING id, device_id, admin_user_id, name, topic, payload_text, qos, retain,
  schedule_type, run_at, time_of_day, weekdays, timezone, status, next_run_at,
  last_run_at, last_error, run_count, created_at, updated_at,
  (SELECT client_id FROM devices WHERE devices.id=scheduled_publish_tasks.device_id)`,
		task.ID, task.DeviceID, task.AdminUserID, task.Name, task.Topic, task.PayloadText, int(task.QoS), task.Retain,
		task.ScheduleType, task.RunAt, task.TimeOfDay, domain.EncodeWeekdays(task.Weekdays), task.Timezone, task.Status, task.NextRunAt, task.CreatedAt)
	return scanScheduledTask(row)
}

func (s *Store) ListScheduledTasks(ctx context.Context, f domain.ScheduledPublishFilter) (domain.Page[domain.ScheduledPublishTaskDTO], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT id, device_id, admin_user_id, name, topic, payload_text, qos, retain,
  schedule_type, run_at, time_of_day, weekdays, timezone, status, next_run_at,
  last_run_at, last_error, run_count, created_at, updated_at, devices.client_id,
  count(*) OVER()
FROM scheduled_publish_tasks
JOIN devices ON devices.id=scheduled_publish_tasks.device_id
WHERE ($1='' OR device_id=$1)
  AND ($2='' OR scheduled_publish_tasks.status=$2)
ORDER BY scheduled_publish_tasks.created_at DESC
LIMIT $3 OFFSET $4`,
		f.DeviceID, f.Status, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.ScheduledPublishTaskDTO]{}, err
	}
	defer rows.Close()
	items := []domain.ScheduledPublishTaskDTO{}
	total := 0
	for rows.Next() {
		task, rowTotal, err := scanScheduledTaskWithTotal(rows)
		if err != nil {
			return domain.Page[domain.ScheduledPublishTaskDTO]{}, err
		}
		items = append(items, task)
		total = rowTotal
	}
	return domain.Page[domain.ScheduledPublishTaskDTO]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func (s *Store) CancelScheduledTask(ctx context.Context, id string, updatedAt time.Time) (domain.ScheduledPublishTaskDTO, error) {
	row := s.pool.QueryRow(ctx, `
UPDATE scheduled_publish_tasks
SET status='canceled', next_run_at=NULL, updated_at=$2
WHERE id=$1
RETURNING id, device_id, admin_user_id, name, topic, payload_text, qos, retain,
  schedule_type, run_at, time_of_day, weekdays, timezone, status, next_run_at,
  last_run_at, last_error, run_count, created_at, updated_at,
  (SELECT client_id FROM devices WHERE devices.id=scheduled_publish_tasks.device_id)`,
		id, updatedAt)
	return scanScheduledTask(row)
}

func (s *Store) ListDueScheduledTasks(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledPublishTaskDTO, error) {
	rows, err := s.pool.Query(ctx, `
SELECT id, device_id, admin_user_id, name, topic, payload_text, qos, retain,
  schedule_type, run_at, time_of_day, weekdays, timezone, status, next_run_at,
  last_run_at, last_error, run_count, created_at, updated_at,
  (SELECT client_id FROM devices WHERE devices.id=scheduled_publish_tasks.device_id)
FROM scheduled_publish_tasks
WHERE status='active'
  AND next_run_at IS NOT NULL
  AND next_run_at <= $1
ORDER BY next_run_at ASC
LIMIT $2`,
		now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks := []domain.ScheduledPublishTaskDTO{}
	for rows.Next() {
		task, err := scanScheduledTask(rows)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) FinishScheduledRun(ctx context.Context, result domain.ScheduledPublishFinish) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
INSERT INTO scheduled_publish_runs (id, task_id, publish_command_id, status, error, scheduled_for, started_at, finished_at)
VALUES ($1, $2, NULLIF($3, ''), $4, $5, $6, $7, $8)`,
		result.RunID, result.TaskID, result.PublishCommandID, result.RunStatus, result.Error, result.ScheduledFor, result.StartedAt, result.FinishedAt); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
UPDATE scheduled_publish_tasks
SET status=$2,
    next_run_at=$3,
    last_run_at=$4,
    last_error=$5,
    run_count=run_count+1,
    updated_at=$4
WHERE id=$1
  AND status='active'`,
		result.TaskID, result.TaskStatus, result.NextRunAt, result.FinishedAt, result.Error); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) UpsertAlert(ctx context.Context, alert domain.SystemAlert) (domain.SystemAlert, error) {
	row := s.pool.QueryRow(ctx, `
INSERT INTO system_alerts (id, level, code, message, status, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (code) DO UPDATE
SET level=EXCLUDED.level,
    message=EXCLUDED.message,
    status='open',
    last_seen_at=EXCLUDED.last_seen_at
RETURNING id, level, code, message, status, first_seen_at, last_seen_at`,
		alert.ID, alert.Level, alert.Code, alert.Message, alert.Status, alert.FirstSeenAt, alert.LastSeenAt)
	return scanAlert(row)
}

func (s *Store) ListAlerts(ctx context.Context, f domain.AlertFilter) (domain.Page[domain.SystemAlert], error) {
	f.Page, f.PageSize = domain.NormalizePage(f.Page, f.PageSize)
	offset := (f.Page - 1) * f.PageSize
	rows, err := s.pool.Query(ctx, `
SELECT id, level, code, message, status, first_seen_at, last_seen_at, count(*) OVER()
FROM system_alerts
WHERE ($1='' OR level=$1)
  AND ($2='' OR status=$2)
ORDER BY last_seen_at DESC
LIMIT $3 OFFSET $4`,
		f.Level, f.Status, f.PageSize, offset)
	if err != nil {
		return domain.Page[domain.SystemAlert]{}, err
	}
	defer rows.Close()
	items := []domain.SystemAlert{}
	total := 0
	for rows.Next() {
		item, rowTotal, err := scanAlertWithTotal(rows)
		if err != nil {
			return domain.Page[domain.SystemAlert]{}, err
		}
		items = append(items, item)
		total = rowTotal
	}
	return domain.Page[domain.SystemAlert]{Items: items, Page: f.Page, PageSize: f.PageSize, Total: total}, rows.Err()
}

func (s *Store) FindAdminByUsername(ctx context.Context, username string) (domain.AdminUser, error) {
	var user domain.AdminUser
	err := s.pool.QueryRow(ctx, `SELECT id, username, password_hash, role, disabled FROM admin_users WHERE username=$1`, username).
		Scan(&user.ID, &user.Username, &user.PasswordHash, &user.Role, &user.Disabled)
	return user, err
}

func (s *Store) GetAdminByID(ctx context.Context, id string) (domain.AdminUserDTO, error) {
	var user domain.AdminUserDTO
	err := s.pool.QueryRow(ctx, `SELECT id, username, role FROM admin_users WHERE id=$1 AND disabled=false`, id).
		Scan(&user.ID, &user.Username, &user.Role)
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, session domain.AdminSession) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO admin_sessions (token, user_id, expires_at)
VALUES ($1, $2, $3)`,
		session.Token, session.UserID, session.ExpiresAt)
	return err
}

func (s *Store) DeleteSession(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `DELETE FROM admin_sessions WHERE token=$1`, token)
	return err
}

func (s *Store) FindSession(ctx context.Context, token string, now time.Time) (domain.AdminSession, error) {
	var session domain.AdminSession
	err := s.pool.QueryRow(ctx, `
SELECT token, user_id, expires_at FROM admin_sessions
WHERE token=$1 AND expires_at>$2`,
		token, now).Scan(&session.Token, &session.UserID, &session.ExpiresAt)
	return session, err
}

func (s *Store) BootstrapAdmin(ctx context.Context, id, username string, passwordHash []byte, now time.Time) error {
	_, err := s.pool.Exec(ctx, `
INSERT INTO admin_users (id, username, password_hash, role, created_at)
VALUES ($1, $2, $3, 'admin', $4)
ON CONFLICT (username) DO NOTHING`,
		id, username, passwordHash, now)
	return err
}

func (s *Store) EnsureMessagePartition(ctx context.Context, at time.Time) error {
	start := time.Date(at.UTC().Year(), at.UTC().Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	name := fmt.Sprintf("mqtt_messages_%04d_%02d", start.Year(), int(start.Month()))
	sqlText := fmt.Sprintf(`
CREATE TABLE IF NOT EXISTS %s PARTITION OF mqtt_messages
FOR VALUES FROM ('%s') TO ('%s');
CREATE INDEX IF NOT EXISTS %s_topic_time ON %s(topic, received_at DESC);
CREATE INDEX IF NOT EXISTS %s_device_time ON %s(device_id, received_at DESC);`,
		name, start.Format(time.RFC3339), end.Format(time.RFC3339), name, name, name, name)
	_, err := s.pool.Exec(ctx, sqlText)
	return err
}

func (s *Store) DeleteMessagesBefore(ctx context.Context, before time.Time) (int64, error) {
	tag, err := s.pool.Exec(ctx, `DELETE FROM mqtt_messages WHERE received_at < $1`, before)
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

func ensureDevice(ctx context.Context, tx pgx.Tx, clientID, sessionID string, at time.Time) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `
INSERT INTO devices (id, client_id, type, status, session_id, first_seen_at, last_seen_at)
VALUES (gen_random_uuid()::text, $1, 'unknown', 'online', $2, $3, $3)
ON CONFLICT (client_id) DO UPDATE
SET session_id=EXCLUDED.session_id,
    last_seen_at=EXCLUDED.last_seen_at
RETURNING id`,
		clientID, sessionID, at).Scan(&id)
	return id, err
}

type scanner interface {
	Scan(dest ...any) error
}

func scanDevice(row scanner) (domain.DeviceDTO, error) {
	var d domain.DeviceDTO
	var metadata []byte
	var lastDisconnected sql.NullTime
	err := row.Scan(&d.ID, &d.ClientID, &d.Type, &d.Status, &d.SessionID, &d.FirstSeenAt, &d.LastSeenAt, &lastDisconnected, &metadata)
	if err != nil {
		return domain.DeviceDTO{}, err
	}
	if lastDisconnected.Valid {
		d.LastDisconnectedAt = &lastDisconnected.Time
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &d.Metadata)
	}
	return d, nil
}

func scanDeviceWithTotal(row scanner) (domain.DeviceDTO, int, error) {
	var d domain.DeviceDTO
	var metadata []byte
	var lastDisconnected sql.NullTime
	var total int
	err := row.Scan(&d.ID, &d.ClientID, &d.Type, &d.Status, &d.SessionID, &d.FirstSeenAt, &d.LastSeenAt, &lastDisconnected, &metadata, &total)
	if err != nil {
		return domain.DeviceDTO{}, 0, err
	}
	if lastDisconnected.Valid {
		d.LastDisconnectedAt = &lastDisconnected.Time
	}
	if len(metadata) > 0 {
		_ = json.Unmarshal(metadata, &d.Metadata)
	}
	return d, total, nil
}

func scanTopicPage(rows pgx.Rows, page, pageSize int) (domain.Page[domain.ObservedTopicDTO], error) {
	items := []domain.ObservedTopicDTO{}
	total := 0
	for rows.Next() {
		var item domain.ObservedTopicDTO
		var deviceID sql.NullString
		var qos int
		var rowTotal int
		if err := rows.Scan(&deviceID, &item.ClientID, &item.Topic, &item.Kind, &qos, &item.FirstSeenAt, &item.LastSeenAt, &rowTotal); err != nil {
			return domain.Page[domain.ObservedTopicDTO]{}, err
		}
		if deviceID.Valid {
			item.DeviceID = deviceID.String
		}
		item.QoS = byte(qos)
		items = append(items, item)
		total = rowTotal
	}
	return domain.Page[domain.ObservedTopicDTO]{Items: items, Page: page, PageSize: pageSize, Total: total}, rows.Err()
}

func scanMessage(row scanner) (domain.MessageDTO, error) {
	var msg domain.MessageDTO
	var deviceID sql.NullString
	var qos int
	err := row.Scan(&msg.ID, &deviceID, &msg.ClientID, &msg.SessionID, &msg.Topic, &msg.PayloadText, &msg.PayloadFormat, &qos, &msg.Retain, &msg.ReceivedAt)
	if err != nil {
		return domain.MessageDTO{}, err
	}
	if deviceID.Valid {
		msg.DeviceID = deviceID.String
	}
	msg.QoS = byte(qos)
	return msg, nil
}

func scanMessageWithTotal(row scanner) (domain.MessageDTO, int, error) {
	var msg domain.MessageDTO
	var deviceID sql.NullString
	var qos int
	var total int
	err := row.Scan(&msg.ID, &deviceID, &msg.ClientID, &msg.SessionID, &msg.Topic, &msg.PayloadText, &msg.PayloadFormat, &qos, &msg.Retain, &msg.ReceivedAt, &total)
	if err != nil {
		return domain.MessageDTO{}, 0, err
	}
	if deviceID.Valid {
		msg.DeviceID = deviceID.String
	}
	msg.QoS = byte(qos)
	return msg, total, nil
}

func scanCommand(row scanner) (domain.PublishCommandDTO, error) {
	var cmd domain.PublishCommandDTO
	var qos int
	var publishedAt sql.NullTime
	err := row.Scan(&cmd.ID, &cmd.AdminUserID, &cmd.Topic, &cmd.PayloadText, &qos, &cmd.Retain, &cmd.Status, &cmd.Error, &cmd.CreatedAt, &publishedAt)
	if err != nil {
		return domain.PublishCommandDTO{}, err
	}
	cmd.QoS = byte(qos)
	if publishedAt.Valid {
		cmd.PublishedAt = &publishedAt.Time
	}
	return cmd, nil
}

func scanCommandWithTotal(row scanner) (domain.PublishCommandDTO, int, error) {
	var cmd domain.PublishCommandDTO
	var qos int
	var publishedAt sql.NullTime
	var total int
	err := row.Scan(&cmd.ID, &cmd.AdminUserID, &cmd.Topic, &cmd.PayloadText, &qos, &cmd.Retain, &cmd.Status, &cmd.Error, &cmd.CreatedAt, &publishedAt, &total)
	if err != nil {
		return domain.PublishCommandDTO{}, 0, err
	}
	cmd.QoS = byte(qos)
	if publishedAt.Valid {
		cmd.PublishedAt = &publishedAt.Time
	}
	return cmd, total, nil
}

func scanScheduledTask(row scanner) (domain.ScheduledPublishTaskDTO, error) {
	var task domain.ScheduledPublishTaskDTO
	var qos int
	var runAt sql.NullTime
	var nextRunAt sql.NullTime
	var lastRunAt sql.NullTime
	var weekdays string
	err := row.Scan(
		&task.ID, &task.DeviceID, &task.AdminUserID, &task.Name, &task.Topic, &task.PayloadText,
		&qos, &task.Retain, &task.ScheduleType, &runAt, &task.TimeOfDay, &weekdays, &task.Timezone,
		&task.Status, &nextRunAt, &lastRunAt, &task.LastError, &task.RunCount, &task.CreatedAt, &task.UpdatedAt,
		&task.ClientID,
	)
	if err != nil {
		return domain.ScheduledPublishTaskDTO{}, err
	}
	task.QoS = byte(qos)
	task.Weekdays = domain.DecodeWeekdays(weekdays)
	if runAt.Valid {
		task.RunAt = &runAt.Time
	}
	if nextRunAt.Valid {
		task.NextRunAt = &nextRunAt.Time
	}
	if lastRunAt.Valid {
		task.LastRunAt = &lastRunAt.Time
	}
	return task, nil
}

func scanScheduledTaskWithTotal(row scanner) (domain.ScheduledPublishTaskDTO, int, error) {
	var task domain.ScheduledPublishTaskDTO
	var qos int
	var runAt sql.NullTime
	var nextRunAt sql.NullTime
	var lastRunAt sql.NullTime
	var weekdays string
	var total int
	err := row.Scan(
		&task.ID, &task.DeviceID, &task.AdminUserID, &task.Name, &task.Topic, &task.PayloadText,
		&qos, &task.Retain, &task.ScheduleType, &runAt, &task.TimeOfDay, &weekdays, &task.Timezone,
		&task.Status, &nextRunAt, &lastRunAt, &task.LastError, &task.RunCount, &task.CreatedAt, &task.UpdatedAt,
		&task.ClientID, &total,
	)
	if err != nil {
		return domain.ScheduledPublishTaskDTO{}, 0, err
	}
	task.QoS = byte(qos)
	task.Weekdays = domain.DecodeWeekdays(weekdays)
	if runAt.Valid {
		task.RunAt = &runAt.Time
	}
	if nextRunAt.Valid {
		task.NextRunAt = &nextRunAt.Time
	}
	if lastRunAt.Valid {
		task.LastRunAt = &lastRunAt.Time
	}
	return task, total, nil
}

func scanAlert(row scanner) (domain.SystemAlert, error) {
	var alert domain.SystemAlert
	err := row.Scan(&alert.ID, &alert.Level, &alert.Code, &alert.Message, &alert.Status, &alert.FirstSeenAt, &alert.LastSeenAt)
	return alert, err
}

func scanAlertWithTotal(row scanner) (domain.SystemAlert, int, error) {
	var alert domain.SystemAlert
	var total int
	err := row.Scan(&alert.ID, &alert.Level, &alert.Code, &alert.Message, &alert.Status, &alert.FirstSeenAt, &alert.LastSeenAt, &total)
	return alert, total, err
}
