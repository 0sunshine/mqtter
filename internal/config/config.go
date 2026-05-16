package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	DatabaseURL       string
	HTTPAddr          string
	MQTTAddr          string
	BrokerStorePath   string
	IngestTimeout     time.Duration
	SchedulerInterval time.Duration
	MaxPayloadBytes   int
	SessionCookieName string
	SessionTTL        time.Duration
	BootstrapUsername string
	BootstrapPassword string
}

func LoadFromEnv() Config {
	return Config{
		DatabaseURL:       getenv("MQTTER_DATABASE_URL", ""),
		HTTPAddr:          getenv("MQTTER_HTTP_ADDR", ":8080"),
		MQTTAddr:          getenv("MQTTER_MQTT_ADDR", ":1883"),
		BrokerStorePath:   getenv("MQTTER_BROKER_STORE_PATH", "data/broker-pebble"),
		IngestTimeout:     durationEnv("MQTTER_INGEST_TIMEOUT", 2*time.Second),
		SchedulerInterval: durationEnv("MQTTER_SCHEDULER_INTERVAL", 10*time.Second),
		MaxPayloadBytes:   intEnv("MQTTER_MAX_PAYLOAD_BYTES", 256*1024),
		SessionCookieName: getenv("MQTTER_SESSION_COOKIE", "mqtter_session"),
		SessionTTL:        durationEnv("MQTTER_SESSION_TTL", 24*time.Hour),
		BootstrapUsername: getenv("MQTTER_BOOTSTRAP_ADMIN_USERNAME", ""),
		BootstrapPassword: getenv("MQTTER_BOOTSTRAP_ADMIN_PASSWORD", ""),
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func intEnv(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func durationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}
