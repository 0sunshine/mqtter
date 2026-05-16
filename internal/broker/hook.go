package broker

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	mqtt "github.com/mochi-mqtt/server/v2"
	"github.com/mochi-mqtt/server/v2/packets"

	"mqtter/internal/domain"
	"mqtter/internal/ports"
	"mqtter/internal/service"
)

type BrokerIngestor interface {
	EnsureConnected(ctx context.Context, ev domain.ConnectEvent) error
	RecordSubscribe(ctx context.Context, ev domain.SubscribeEvent) error
	PersistBeforeRoute(ctx context.Context, ev domain.PublishEvent) error
	MarkDisconnected(ctx context.Context, ev domain.DisconnectEvent) error
}

type Hook struct {
	mqtt.HookBase

	ingestor BrokerIngestor
	clock    ports.Clock
	ids      ports.IDGenerator

	mu             sync.RWMutex
	sessions       map[*mqtt.Client]string
	connectLimiter *connectLimiter
}

func NewHook(ingestor BrokerIngestor, clock ports.Clock, ids ports.IDGenerator) *Hook {
	if clock == nil {
		clock = service.SystemClock{}
	}
	if ids == nil {
		ids = service.RandomIDGenerator{}
	}
	return &Hook{
		ingestor:       ingestor,
		clock:          clock,
		ids:            ids,
		sessions:       map[*mqtt.Client]string{},
		connectLimiter: newConnectLimiter(120, time.Minute),
	}
}

func (h *Hook) ID() string {
	return "mqtter-ingestion"
}

func (h *Hook) Provides(b byte) bool {
	switch b {
	case mqtt.OnConnectAuthenticate,
		mqtt.OnACLCheck,
		mqtt.OnConnect,
		mqtt.OnDisconnect,
		mqtt.OnSubscribe,
		mqtt.OnPublish:
		return true
	default:
		return false
	}
}

func (h *Hook) OnConnectAuthenticate(_ *mqtt.Client, _ packets.Packet) bool {
	return true
}

func (h *Hook) OnACLCheck(_ *mqtt.Client, topic string, write bool) bool {
	if write {
		return domain.ValidatePublishTopic(topic) == nil
	}
	return domain.ValidateSubscribeFilter(topic) == nil
}

func (h *Hook) OnConnect(cl *mqtt.Client, _ packets.Packet) error {
	now := h.clock.Now()
	remote := cl.Net.Remote
	if remote == "" {
		remote = cl.ID
	}
	if !h.connectLimiter.Allow(remote, now) {
		return packets.ErrServerBusy
	}

	sessionID := h.ids.NewID()
	h.mu.Lock()
	h.sessions[cl] = sessionID
	h.mu.Unlock()

	err := h.ingestor.EnsureConnected(context.Background(), domain.ConnectEvent{
		ClientID:        cl.ID,
		Username:        string(cl.Properties.Username),
		Remote:          cl.Net.Remote,
		Listener:        cl.Net.Listener,
		SessionID:       sessionID,
		ProtocolVersion: cl.Properties.ProtocolVersion,
		ConnectedAt:     now,
	})
	if err == nil {
		return nil
	}
	if domain.ErrorCode(err) == "invalid_client_id" {
		return packets.ErrClientIdentifierNotValid
	}
	return packets.ErrServerUnavailable
}

func (h *Hook) OnDisconnect(cl *mqtt.Client, err error, expire bool) {
	sessionID := h.sessionID(cl)
	reason := ""
	if err != nil {
		reason = err.Error()
	}
	_ = h.ingestor.MarkDisconnected(context.Background(), domain.DisconnectEvent{
		ClientID:       cl.ID,
		SessionID:      sessionID,
		Reason:         reason,
		ExpireSession:  expire,
		DisconnectedAt: h.clock.Now(),
	})
	h.mu.Lock()
	delete(h.sessions, cl)
	h.mu.Unlock()
}

func (h *Hook) OnSubscribe(cl *mqtt.Client, pk packets.Packet) packets.Packet {
	subs := make([]domain.Subscription, 0, len(pk.Filters))
	for _, f := range pk.Filters {
		subs = append(subs, domain.Subscription{Filter: f.Filter, QoS: f.Qos})
	}
	err := h.ingestor.RecordSubscribe(context.Background(), domain.SubscribeEvent{
		ClientID:      cl.ID,
		SessionID:     h.sessionID(cl),
		Subscriptions: subs,
		ObservedAt:    h.clock.Now(),
	})
	if err == nil {
		return pk
	}

	for i := range pk.Filters {
		pk.Filters[i].Filter = "#/#"
	}
	return pk
}

func (h *Hook) OnPublish(cl *mqtt.Client, pk packets.Packet) (packets.Packet, error) {
	err := h.ingestor.PersistBeforeRoute(context.Background(), domain.PublishEvent{
		ClientID:    cl.ID,
		SessionID:   h.sessionID(cl),
		Topic:       pk.TopicName,
		PayloadText: string(pk.Payload),
		QoS:         pk.FixedHeader.Qos,
		Retain:      pk.FixedHeader.Retain,
		ReceivedAt:  h.clock.Now(),
	})
	if err == nil {
		return pk, nil
	}
	code := domain.ErrorCode(err)
	if code == "unsupported_payload" {
		if pk.FixedHeader.Qos > 0 {
			return pk, packets.ErrPayloadFormatInvalid
		}
		return pk, packets.ErrRejectPacket
	}
	if errors.Is(err, context.DeadlineExceeded) || !strings.HasPrefix(code, "invalid_") {
		if pk.FixedHeader.Qos > 0 {
			return pk, packets.ErrServerBusy
		}
		return pk, packets.ErrRejectPacket
	}
	return pk, packets.ErrRejectPacket
}

func (h *Hook) sessionID(cl *mqtt.Client) string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sessions[cl]
}
