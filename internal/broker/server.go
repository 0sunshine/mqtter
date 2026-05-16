package broker

import (
	"context"
	"log/slog"
	"os"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/hooks/storage/pebble"
	"github.com/mochi-mqtt/server/v2/listeners"

	"mqtter/internal/domain"
)

type Config struct {
	MQTTAddr        string
	StorePath       string
	MaxPayloadBytes int
	MaxClients      int64
}

type Server struct {
	mqtt *mqtt.Server
}

func NewServer(cfg Config, ingestor BrokerIngestor, logger *slog.Logger) (*Server, error) {
	if cfg.MQTTAddr == "" {
		cfg.MQTTAddr = ":1883"
	}
	if cfg.StorePath == "" {
		cfg.StorePath = "data/broker-pebble"
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(os.Stdout, nil))
	}
	if err := os.MkdirAll(cfg.StorePath, 0o755); err != nil {
		return nil, err
	}

	caps := mqtt.NewDefaultServerCapabilities()
	caps.MaximumQos = 1
	if cfg.MaxPayloadBytes > 0 {
		caps.MaximumPacketSize = uint32(cfg.MaxPayloadBytes + 1024)
	}
	if cfg.MaxClients > 0 {
		caps.MaximumClients = cfg.MaxClients
	}

	srv := mqtt.New(&mqtt.Options{
		InlineClient: true,
		Capabilities: caps,
		Logger:       logger,
	})
	if err := srv.AddHook(&pebble.Hook{}, &pebble.Options{Path: cfg.StorePath, Mode: pebble.Sync}); err != nil {
		return nil, err
	}
	if err := srv.AddHook(NewHook(ingestor, nil, nil), nil); err != nil {
		return nil, err
	}
	if err := srv.AddListener(listeners.NewTCP(listeners.Config{ID: "tcp", Address: cfg.MQTTAddr})); err != nil {
		return nil, err
	}
	return &Server{mqtt: srv}, nil
}

func (s *Server) Serve() error {
	return s.mqtt.Serve()
}

func (s *Server) Close() error {
	return s.mqtt.Close()
}

func (s *Server) Publish(ctx context.Context, topic string, payload string, opts domain.PublishOptions) error {
	done := make(chan error, 1)
	go func() {
		done <- s.mqtt.Publish(topic, []byte(payload), opts.Retain, opts.QoS)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}
