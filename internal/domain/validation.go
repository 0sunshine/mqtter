package domain

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const (
	MaxClientIDLength = 128
	MaxTopicLength    = 512
)

var clientIDPattern = regexp.MustCompile(`^[A-Za-z0-9._:@-]+$`)

func ValidateClientID(clientID string) error {
	if clientID == "" {
		return InvalidInput("invalid_client_id", "client id must not be empty")
	}
	if len(clientID) > MaxClientIDLength || !clientIDPattern.MatchString(clientID) {
		return InvalidInput("invalid_client_id", "client id contains unsupported characters or is too long")
	}
	return nil
}

func ValidatePublishTopic(topic string) error {
	if topic == "" {
		return InvalidInput("invalid_topic", "topic must not be empty")
	}
	if len(topic) > MaxTopicLength {
		return InvalidInput("invalid_topic", "topic is too long")
	}
	if strings.ContainsAny(topic, "+#") {
		return InvalidInput("invalid_topic", "publish topic must not contain wildcards")
	}
	if strings.HasPrefix(strings.ToUpper(topic), "$SYS/") || strings.EqualFold(topic, "$SYS") {
		return InvalidInput("invalid_topic", "publishing to $SYS topics is not allowed")
	}
	return nil
}

func ValidateSubscribeFilter(filter string) error {
	if filter == "" {
		return InvalidInput("invalid_topic_filter", "topic filter must not be empty")
	}
	if len(filter) > MaxTopicLength {
		return InvalidInput("invalid_topic_filter", "topic filter is too long")
	}
	if idx := strings.IndexRune(filter, '#'); idx >= 0 && idx != len(filter)-1 {
		return InvalidInput("invalid_topic_filter", "multi-level wildcard must be the final character")
	}
	return nil
}

func ValidateTextPayload(payload string, maxBytes int) (PayloadFormat, error) {
	if maxBytes > 0 && len([]byte(payload)) > maxBytes {
		return "", InvalidInput("payload_too_large", "payload exceeds maximum allowed size")
	}
	if !utf8.ValidString(payload) {
		return "", InvalidInput("unsupported_payload", "payload must be valid UTF-8 text")
	}
	format := PayloadFormatText
	var js any
	if json.Unmarshal([]byte(payload), &js) == nil {
		format = PayloadFormatJSON
	}
	return format, nil
}

func ValidateQoS(qos byte) error {
	if qos > 1 {
		return InvalidInput("invalid_qos", "qos must be 0 or 1")
	}
	return nil
}
