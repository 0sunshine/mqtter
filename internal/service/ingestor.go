package service

import (
	"context"
	"errors"
	"time"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type IngestorConfig struct {
	Timeout         time.Duration
	MaxPayloadBytes int
}

type Ingestor struct {
	repo     ports.IngestionRepository
	alerts   ports.AlertRepository
	realtime ports.RealtimePublisher
	clock    ports.Clock
	config   IngestorConfig
}

func NewIngestor(repo ports.IngestionRepository, alerts ports.AlertRepository, realtime ports.RealtimePublisher, clock ports.Clock, cfg IngestorConfig) *Ingestor {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 2 * time.Second
	}
	if clock == nil {
		clock = SystemClock{}
	}
	return &Ingestor{repo: repo, alerts: alerts, realtime: realtime, clock: clock, config: cfg}
}

func (s *Ingestor) EnsureConnected(ctx context.Context, ev domain.ConnectEvent) error {
	if err := domain.ValidateClientID(ev.ClientID); err != nil {
		return err
	}
	if ev.ConnectedAt.IsZero() {
		ev.ConnectedAt = s.clock.Now()
	}
	return s.withBackpressure(ctx, "device_connect_failed", func(ctx context.Context) error {
		device, err := s.repo.UpsertConnected(ctx, ev)
		if err != nil {
			return err
		}
		s.publishRealtime(ctx, "device.connected", device)
		return nil
	})
}

func (s *Ingestor) RecordSubscribe(ctx context.Context, ev domain.SubscribeEvent) error {
	if err := domain.ValidateClientID(ev.ClientID); err != nil {
		return err
	}
	for _, sub := range ev.Subscriptions {
		if err := domain.ValidateSubscribeFilter(sub.Filter); err != nil {
			return err
		}
		if err := domain.ValidateQoS(sub.QoS); err != nil {
			return err
		}
	}
	if ev.ObservedAt.IsZero() {
		ev.ObservedAt = s.clock.Now()
	}
	return s.withBackpressure(ctx, "subscription_record_failed", func(ctx context.Context) error {
		if err := s.repo.RecordSubscription(ctx, ev); err != nil {
			return err
		}
		s.publishRealtime(ctx, "topic.observed", ev)
		return nil
	})
}

func (s *Ingestor) PersistBeforeRoute(ctx context.Context, ev domain.PublishEvent) error {
	if ev.ClientID != "inline" {
		if err := domain.ValidateClientID(ev.ClientID); err != nil {
			return err
		}
	}
	if err := domain.ValidatePublishTopic(ev.Topic); err != nil {
		return err
	}
	if err := domain.ValidateQoS(ev.QoS); err != nil {
		return err
	}
	format, err := domain.ValidateTextPayload(ev.PayloadText, s.config.MaxPayloadBytes)
	if err != nil {
		return err
	}
	ev.PayloadFormat = format
	if ev.ReceivedAt.IsZero() {
		ev.ReceivedAt = s.clock.Now()
	}
	return s.withBackpressure(ctx, "publish_persist_failed", func(ctx context.Context) error {
		msg, err := s.repo.PersistPublish(ctx, ev)
		if err != nil {
			return err
		}
		s.publishRealtime(ctx, "message.received", msg)
		return nil
	})
}

func (s *Ingestor) MarkDisconnected(ctx context.Context, ev domain.DisconnectEvent) error {
	if ev.ClientID == "" {
		return nil
	}
	if ev.DisconnectedAt.IsZero() {
		ev.DisconnectedAt = s.clock.Now()
	}
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := s.repo.MarkDisconnected(ctx, ev)
	if err != nil {
		_ = s.raise(ctx, domain.AlertLevelWarning, "device_disconnect_record_failed", "failed to record device disconnect: "+err.Error())
		return err
	}
	s.publishRealtime(ctx, "device.disconnected", ev)
	return nil
}

func (s *Ingestor) MarkStaleOnline(ctx context.Context) (int64, error) {
	now := s.clock.Now()
	count, err := s.repo.MarkStaleOnline(ctx, now)
	if err != nil {
		return 0, err
	}
	if count > 0 {
		_ = s.raise(ctx, domain.AlertLevelWarning, "stale_online_devices", "marked stale online devices as offline")
	}
	return count, nil
}

func (s *Ingestor) withBackpressure(ctx context.Context, code string, fn func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(ctx, s.config.Timeout)
	defer cancel()
	err := fn(ctx)
	if err == nil {
		return nil
	}
	level := domain.AlertLevelWarning
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
		level = domain.AlertLevelCritical
	}
	_ = s.raise(context.Background(), level, code, err.Error())
	return err
}

func (s *Ingestor) raise(ctx context.Context, level domain.AlertLevel, code string, message string) error {
	if s.alerts == nil {
		return nil
	}
	now := s.clock.Now()
	alert := domain.SystemAlert{
		ID:          code,
		Level:       level,
		Code:        code,
		Message:     message,
		Status:      domain.AlertStatusOpen,
		FirstSeenAt: now,
		LastSeenAt:  now,
	}
	saved, err := s.alerts.UpsertAlert(ctx, alert)
	if err == nil {
		s.publishRealtime(ctx, "system.alert", saved)
	}
	return err
}

func (s *Ingestor) publishRealtime(ctx context.Context, typ string, data any) {
	if s.realtime == nil {
		return
	}
	_ = s.realtime.Publish(ctx, ports.RealtimeEvent{
		Type: typ,
		Data: data,
		At:   s.clock.Now().Format(time.RFC3339Nano),
	})
}
