package domain

import "time"

type Subscription struct {
	Filter string
	QoS    byte
}

type ConnectEvent struct {
	ClientID        string
	Username        string
	Remote          string
	Listener        string
	SessionID       string
	ProtocolVersion byte
	ConnectedAt     time.Time
}

type SubscribeEvent struct {
	ClientID      string
	SessionID     string
	Subscriptions []Subscription
	ObservedAt    time.Time
}

type PublishEvent struct {
	ClientID      string
	SessionID     string
	Topic         string
	PayloadText   string
	PayloadFormat PayloadFormat
	QoS           byte
	Retain        bool
	ReceivedAt    time.Time
}

type DisconnectEvent struct {
	ClientID       string
	SessionID      string
	Reason         string
	ExpireSession  bool
	DisconnectedAt time.Time
}

type ChangeDeviceTypeCommand struct {
	DeviceID string `json:"-"`
	Type     string `json:"type"`
	ActorID  string `json:"-"`
}

type PublishCommand struct {
	AdminUserID string `json:"-"`
	Topic       string `json:"topic"`
	PayloadText string `json:"payload"`
	QoS         byte   `json:"qos"`
	Retain      bool   `json:"retain"`
}

type PublishOptions struct {
	QoS    byte
	Retain bool
}

type PublishResult struct {
	CommandID   string        `json:"commandId"`
	Status      PublishStatus `json:"status"`
	PublishedAt time.Time     `json:"publishedAt"`
}
