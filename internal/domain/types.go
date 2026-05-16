package domain

import "time"

type DeviceStatus string

const (
	DeviceStatusOnline       DeviceStatus = "online"
	DeviceStatusOffline      DeviceStatus = "offline"
	DeviceStatusStaleOffline DeviceStatus = "stale_offline"
)

type TopicKind string

const (
	TopicKindSubscribeFilter   TopicKind = "subscribe_filter"
	TopicKindPublishTopic      TopicKind = "publish_topic"
	TopicKindAdminPublishTopic TopicKind = "admin_publish_topic"
)

type PayloadFormat string

const (
	PayloadFormatText PayloadFormat = "text"
	PayloadFormatJSON PayloadFormat = "json"
)

type PublishStatus string

const (
	PublishStatusPending   PublishStatus = "pending"
	PublishStatusPublished PublishStatus = "published"
	PublishStatusFailed    PublishStatus = "failed"
)

type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

type AlertStatus string

const (
	AlertStatusOpen     AlertStatus = "open"
	AlertStatusResolved AlertStatus = "resolved"
)

type Page[T any] struct {
	Items    []T `json:"items"`
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
	Total    int `json:"total"`
}

type DeviceDTO struct {
	ID                 string         `json:"id"`
	ClientID           string         `json:"clientId"`
	Type               string         `json:"type"`
	Status             DeviceStatus   `json:"status"`
	SessionID          string         `json:"sessionId,omitempty"`
	FirstSeenAt        time.Time      `json:"firstSeenAt"`
	LastSeenAt         time.Time      `json:"lastSeenAt"`
	LastDisconnectedAt *time.Time     `json:"lastDisconnectedAt,omitempty"`
	Metadata           map[string]any `json:"metadata,omitempty"`
}

type ObservedTopicDTO struct {
	DeviceID    string    `json:"deviceId,omitempty"`
	ClientID    string    `json:"clientId"`
	Topic       string    `json:"topic"`
	Kind        TopicKind `json:"kind"`
	QoS         byte      `json:"qos"`
	FirstSeenAt time.Time `json:"firstSeenAt"`
	LastSeenAt  time.Time `json:"lastSeenAt"`
}

type MessageDTO struct {
	ID            string        `json:"id"`
	DeviceID      string        `json:"deviceId,omitempty"`
	ClientID      string        `json:"clientId"`
	SessionID     string        `json:"sessionId,omitempty"`
	Topic         string        `json:"topic"`
	PayloadText   string        `json:"payloadText"`
	PayloadFormat PayloadFormat `json:"payloadFormat"`
	QoS           byte          `json:"qos"`
	Retain        bool          `json:"retain"`
	ReceivedAt    time.Time     `json:"receivedAt"`
}

type PublishCommandDTO struct {
	ID          string        `json:"id"`
	AdminUserID string        `json:"adminUserId"`
	Topic       string        `json:"topic"`
	PayloadText string        `json:"payloadText"`
	QoS         byte          `json:"qos"`
	Retain      bool          `json:"retain"`
	Status      PublishStatus `json:"status"`
	Error       string        `json:"error,omitempty"`
	CreatedAt   time.Time     `json:"createdAt"`
	PublishedAt *time.Time    `json:"publishedAt,omitempty"`
}

type DeviceTypeDTO struct {
	Code        string    `json:"code"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Schema      string    `json:"schema,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

type AdminUserDTO struct {
	ID       string `json:"id"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type AdminUser struct {
	ID           string
	Username     string
	PasswordHash []byte
	Role         string
	Disabled     bool
}

type AdminSession struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

type SystemAlert struct {
	ID          string      `json:"id"`
	Level       AlertLevel  `json:"level"`
	Code        string      `json:"code"`
	Message     string      `json:"message"`
	Status      AlertStatus `json:"status"`
	FirstSeenAt time.Time   `json:"firstSeenAt"`
	LastSeenAt  time.Time   `json:"lastSeenAt"`
}
