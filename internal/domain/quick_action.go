package domain

import (
	"strings"
	"time"
)

const InfraredControllerType = "GSCU1B"

type CreateQuickActionCommand struct {
	AdminUserID string `json:"-"`
	DeviceID    string `json:"deviceId"`
	Name        string `json:"name"`
	Topic       string `json:"topic"`
	PayloadText string `json:"payload"`
	QoS         byte   `json:"qos"`
	Retain      bool   `json:"retain"`
}

type QuickActionDTO struct {
	ID          string    `json:"id"`
	DeviceID    string    `json:"deviceId"`
	ClientID    string    `json:"clientId,omitempty"`
	AdminUserID string    `json:"adminUserId"`
	Name        string    `json:"name"`
	Topic       string    `json:"topic"`
	PayloadText string    `json:"payload"`
	QoS         byte      `json:"qos"`
	Retain      bool      `json:"retain"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type QuickActionFilter struct {
	DeviceID string
	Page     int
	PageSize int
}

type QuickActionExecuteResult struct {
	ActionID    string        `json:"actionId"`
	CommandID   string        `json:"commandId"`
	Status      PublishStatus `json:"status"`
	PublishedAt time.Time     `json:"publishedAt"`
}

func ValidateQuickActionCommand(cmd CreateQuickActionCommand, maxPayload int) error {
	if strings.TrimSpace(cmd.DeviceID) == "" {
		return InvalidInput("invalid_device_id", "device id must not be empty")
	}
	if strings.TrimSpace(cmd.Name) == "" {
		return InvalidInput("invalid_quick_action_name", "quick action name must not be empty")
	}
	if len([]rune(strings.TrimSpace(cmd.Name))) > 80 {
		return InvalidInput("invalid_quick_action_name", "quick action name is too long")
	}
	if err := ValidatePublishTopic(cmd.Topic); err != nil {
		return err
	}
	if err := ValidateQoS(cmd.QoS); err != nil {
		return err
	}
	if _, err := ValidateTextPayload(cmd.PayloadText, maxPayload); err != nil {
		return err
	}
	return nil
}
