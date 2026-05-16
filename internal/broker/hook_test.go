package broker

import (
	"context"
	"errors"
	"testing"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	"mqtter/internal/domain"
)

type fakeBrokerIngestor struct {
	connects   []domain.ConnectEvent
	subscribes []domain.SubscribeEvent
	publishes  []domain.PublishEvent
	subErr     error
	pubErr     error
}

func (i *fakeBrokerIngestor) EnsureConnected(_ context.Context, ev domain.ConnectEvent) error {
	i.connects = append(i.connects, ev)
	return nil
}

func (i *fakeBrokerIngestor) RecordSubscribe(_ context.Context, ev domain.SubscribeEvent) error {
	i.subscribes = append(i.subscribes, ev)
	return i.subErr
}

func (i *fakeBrokerIngestor) PersistBeforeRoute(_ context.Context, ev domain.PublishEvent) error {
	i.publishes = append(i.publishes, ev)
	return i.pubErr
}

func (i *fakeBrokerIngestor) MarkDisconnected(context.Context, domain.DisconnectEvent) error {
	return nil
}

type brokerFixedIDs struct{}

func (brokerFixedIDs) NewID() string { return "session-1" }

type brokerFixedClock struct{ t time.Time }

func (c brokerFixedClock) Now() time.Time { return c.t }

func TestHookConvertsPublishPacketToIngestionEvent(t *testing.T) {
	ingestor := &fakeBrokerIngestor{}
	hook := NewHook(ingestor, brokerFixedClock{t: time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)}, brokerFixedIDs{})
	client := &mqtt.Client{ID: "device-1"}

	if err := hook.OnConnect(client, packets.Packet{}); err != nil {
		t.Fatalf("OnConnect returned error: %v", err)
	}
	_, err := hook.OnPublish(client, packets.Packet{
		TopicName: "sensors/temp",
		Payload:   []byte("23"),
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
	})
	if err != nil {
		t.Fatalf("OnPublish returned error: %v", err)
	}
	if len(ingestor.publishes) != 1 {
		t.Fatalf("expected one publish event, got %d", len(ingestor.publishes))
	}
	if ingestor.publishes[0].SessionID != "session-1" {
		t.Fatalf("expected session id to be carried, got %q", ingestor.publishes[0].SessionID)
	}
}

func TestHookRejectsSubscribePacketWhenIngestionFails(t *testing.T) {
	ingestor := &fakeBrokerIngestor{subErr: errors.New("db down")}
	hook := NewHook(ingestor, brokerFixedClock{t: time.Now().UTC()}, brokerFixedIDs{})
	client := &mqtt.Client{ID: "device-1"}

	pk := hook.OnSubscribe(client, packets.Packet{
		Filters: packets.Subscriptions{{Filter: "devices/a/#", Qos: 0}},
	})
	if pk.Filters[0].Filter != "#/#" {
		t.Fatalf("expected invalid replacement filter, got %q", pk.Filters[0].Filter)
	}
}

func TestHookReturnsServerBusyForQoSOnePublishWhenPersistenceFails(t *testing.T) {
	ingestor := &fakeBrokerIngestor{pubErr: errors.New("postgres unavailable")}
	hook := NewHook(ingestor, brokerFixedClock{t: time.Now().UTC()}, brokerFixedIDs{})
	client := &mqtt.Client{ID: "device-1"}

	_, err := hook.OnPublish(client, packets.Packet{
		TopicName: "sensors/temp",
		Payload:   []byte("23"),
		FixedHeader: packets.FixedHeader{
			Qos: 1,
		},
	})
	if !errors.Is(err, packets.ErrServerBusy) {
		t.Fatalf("expected server busy error, got %v", err)
	}
}

func TestConnectLimiterRejectsAfterLimit(t *testing.T) {
	limiter := newConnectLimiter(2, time.Minute)
	now := time.Date(2026, 5, 15, 13, 0, 0, 0, time.UTC)
	if !limiter.Allow("127.0.0.1", now) {
		t.Fatal("first connection should be allowed")
	}
	if !limiter.Allow("127.0.0.1", now.Add(time.Second)) {
		t.Fatal("second connection should be allowed")
	}
	if limiter.Allow("127.0.0.1", now.Add(2*time.Second)) {
		t.Fatal("third connection should be rejected")
	}
	if !limiter.Allow("127.0.0.1", now.Add(2*time.Minute)) {
		t.Fatal("connection after window should be allowed")
	}
}
