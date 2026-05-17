package ports

import (
	"context"
	"time"

	"mqtter/internal/domain"
)

type Clock interface {
	Now() time.Time
}

type IDGenerator interface {
	NewID() string
}

type PasswordHasher interface {
	Hash(password string) ([]byte, error)
	Compare(hash []byte, password string) error
}

type IngestionRepository interface {
	UpsertConnected(ctx context.Context, ev domain.ConnectEvent) (domain.DeviceDTO, error)
	RecordSubscription(ctx context.Context, ev domain.SubscribeEvent) error
	PersistPublish(ctx context.Context, ev domain.PublishEvent) (domain.MessageDTO, error)
	MarkDisconnected(ctx context.Context, ev domain.DisconnectEvent) error
	MarkStaleOnline(ctx context.Context, at time.Time) (int64, error)
}

type DeviceRepository interface {
	ListDevices(ctx context.Context, f domain.DeviceFilter) (domain.Page[domain.DeviceDTO], error)
	GetDevice(ctx context.Context, id string) (domain.DeviceDTO, error)
	ChangeDeviceType(ctx context.Context, cmd domain.ChangeDeviceTypeCommand, changedAt time.Time) (domain.DeviceDTO, error)
	ListDeviceTypes(ctx context.Context) ([]domain.DeviceTypeDTO, error)
}

type TopicRepository interface {
	ListDeviceTopics(ctx context.Context, deviceID string, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error)
	ListTopics(ctx context.Context, f domain.TopicFilter) (domain.Page[domain.ObservedTopicDTO], error)
	RecordAdminPublishTopic(ctx context.Context, clientID, topic string, qos byte, at time.Time) error
}

type MessageRepository interface {
	QueryMessages(ctx context.Context, f domain.MessageFilter) (domain.Page[domain.MessageDTO], error)
}

type PublishCommandRepository interface {
	CreatePublishCommand(ctx context.Context, id string, cmd domain.PublishCommand, createdAt time.Time) (domain.PublishCommandDTO, error)
	MarkPublishCommand(ctx context.Context, id string, status domain.PublishStatus, errText string, publishedAt *time.Time) error
	ListPublishCommands(ctx context.Context, f domain.CommandFilter) (domain.Page[domain.PublishCommandDTO], error)
}

type ScheduledPublishRepository interface {
	CreateScheduledTask(ctx context.Context, task domain.ScheduledPublishTaskDTO) (domain.ScheduledPublishTaskDTO, error)
	ListScheduledTasks(ctx context.Context, f domain.ScheduledPublishFilter) (domain.Page[domain.ScheduledPublishTaskDTO], error)
	CancelScheduledTask(ctx context.Context, id string, updatedAt time.Time) (domain.ScheduledPublishTaskDTO, error)
	ListDueScheduledTasks(ctx context.Context, now time.Time, limit int) ([]domain.ScheduledPublishTaskDTO, error)
	FinishScheduledRun(ctx context.Context, result domain.ScheduledPublishFinish) error
}

type QuickActionRepository interface {
	CreateQuickAction(ctx context.Context, action domain.QuickActionDTO) (domain.QuickActionDTO, error)
	ListQuickActions(ctx context.Context, f domain.QuickActionFilter) (domain.Page[domain.QuickActionDTO], error)
	GetQuickAction(ctx context.Context, id string) (domain.QuickActionDTO, error)
	DeleteQuickAction(ctx context.Context, id string) error
}

type AlertRepository interface {
	UpsertAlert(ctx context.Context, alert domain.SystemAlert) (domain.SystemAlert, error)
	ListAlerts(ctx context.Context, f domain.AlertFilter) (domain.Page[domain.SystemAlert], error)
}

type AuthRepository interface {
	FindAdminByUsername(ctx context.Context, username string) (domain.AdminUser, error)
	GetAdminByID(ctx context.Context, id string) (domain.AdminUserDTO, error)
	CreateSession(ctx context.Context, session domain.AdminSession) error
	DeleteSession(ctx context.Context, token string) error
	FindSession(ctx context.Context, token string, now time.Time) (domain.AdminSession, error)
	BootstrapAdmin(ctx context.Context, id, username string, passwordHash []byte, now time.Time) error
}

type MQTTPublisher interface {
	Publish(ctx context.Context, topic string, payload string, opts domain.PublishOptions) error
}

type RealtimePublisher interface {
	Publish(ctx context.Context, event RealtimeEvent) error
}

type RealtimeEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
	At   string `json:"at"`
}
