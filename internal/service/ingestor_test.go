package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
)

type fixedClock struct{ t time.Time }

func (c fixedClock) Now() time.Time { return c.t }

type fakeIngestionRepo struct {
	connected  []domain.ConnectEvent
	subscribed []domain.SubscribeEvent
	published  []domain.PublishEvent
	publishErr error
}

func (r *fakeIngestionRepo) UpsertConnected(_ context.Context, ev domain.ConnectEvent) (domain.DeviceDTO, error) {
	r.connected = append(r.connected, ev)
	return domain.DeviceDTO{ID: "dev-1", ClientID: ev.ClientID, Status: domain.DeviceStatusOnline}, nil
}

func (r *fakeIngestionRepo) RecordSubscription(_ context.Context, ev domain.SubscribeEvent) error {
	r.subscribed = append(r.subscribed, ev)
	return nil
}

func (r *fakeIngestionRepo) PersistPublish(_ context.Context, ev domain.PublishEvent) (domain.MessageDTO, error) {
	if r.publishErr != nil {
		return domain.MessageDTO{}, r.publishErr
	}
	r.published = append(r.published, ev)
	return domain.MessageDTO{ID: "msg-1", ClientID: ev.ClientID, Topic: ev.Topic}, nil
}

func (r *fakeIngestionRepo) MarkDisconnected(context.Context, domain.DisconnectEvent) error {
	return nil
}

func (r *fakeIngestionRepo) MarkStaleOnline(context.Context, time.Time) (int64, error) {
	return 0, nil
}

type fakeAlertRepo struct {
	alerts []domain.SystemAlert
}

func (r *fakeAlertRepo) UpsertAlert(_ context.Context, alert domain.SystemAlert) (domain.SystemAlert, error) {
	r.alerts = append(r.alerts, alert)
	return alert, nil
}

func (r *fakeAlertRepo) ListAlerts(context.Context, domain.AlertFilter) (domain.Page[domain.SystemAlert], error) {
	return domain.Page[domain.SystemAlert]{}, nil
}

type fakeRealtime struct {
	events []ports.RealtimeEvent
}

func (r *fakeRealtime) Publish(_ context.Context, event ports.RealtimeEvent) error {
	r.events = append(r.events, event)
	return nil
}

func TestIngestorPersistBeforeRouteRecordsTextMessage(t *testing.T) {
	repo := &fakeIngestionRepo{}
	alerts := &fakeAlertRepo{}
	rt := &fakeRealtime{}
	clock := fixedClock{t: time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)}
	ingestor := NewIngestor(repo, alerts, rt, clock, IngestorConfig{Timeout: time.Second, MaxPayloadBytes: 1024})

	err := ingestor.PersistBeforeRoute(context.Background(), domain.PublishEvent{
		ClientID:    "device-1",
		SessionID:   "session-1",
		Topic:       "sensors/temp",
		PayloadText: `{"value":23}`,
		QoS:         1,
	})
	if err != nil {
		t.Fatalf("PersistBeforeRoute returned error: %v", err)
	}
	if len(repo.published) != 1 {
		t.Fatalf("expected one persisted publish, got %d", len(repo.published))
	}
	if repo.published[0].PayloadFormat != domain.PayloadFormatJSON {
		t.Fatalf("expected JSON payload format, got %s", repo.published[0].PayloadFormat)
	}
	if len(alerts.alerts) != 0 {
		t.Fatalf("expected no alerts, got %d", len(alerts.alerts))
	}
	if len(rt.events) != 1 || rt.events[0].Type != "message.received" {
		t.Fatalf("expected message realtime event, got %#v", rt.events)
	}
}

func TestIngestorPersistBeforeRouteRaisesAlertOnStoreFailure(t *testing.T) {
	repo := &fakeIngestionRepo{publishErr: errors.New("postgres unavailable")}
	alerts := &fakeAlertRepo{}
	ingestor := NewIngestor(repo, alerts, nil, fixedClock{t: time.Now().UTC()}, IngestorConfig{Timeout: time.Second})

	err := ingestor.PersistBeforeRoute(context.Background(), domain.PublishEvent{
		ClientID:    "device-1",
		Topic:       "sensors/temp",
		PayloadText: "23",
		QoS:         0,
	})
	if err == nil {
		t.Fatal("expected persistence error")
	}
	if len(alerts.alerts) != 1 {
		t.Fatalf("expected one alert, got %d", len(alerts.alerts))
	}
	if alerts.alerts[0].Code != "publish_persist_failed" {
		t.Fatalf("unexpected alert code %s", alerts.alerts[0].Code)
	}
}

func TestIngestorRejectsInvalidSubscriptionBeforeRepository(t *testing.T) {
	repo := &fakeIngestionRepo{}
	ingestor := NewIngestor(repo, nil, nil, fixedClock{t: time.Now().UTC()}, IngestorConfig{Timeout: time.Second})

	err := ingestor.RecordSubscribe(context.Background(), domain.SubscribeEvent{
		ClientID: "device-1",
		Subscriptions: []domain.Subscription{
			{Filter: "bad/#/tail", QoS: 0},
		},
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(repo.subscribed) != 0 {
		t.Fatal("repository should not be called for invalid subscription")
	}
}
