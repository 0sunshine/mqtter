# mqtter

MQTT management backend and embedded broker.

## Backend shape

- Go monolith with modular packages under `internal/`.
- Embedded Mochi MQTT broker with Pebble storage for MQTT retained messages, persistent sessions, subscriptions, and inflight messages.
- PostgreSQL stores devices, observed topics, message history, publish commands, alerts, admin users, and sessions.
- Publish ingestion is synchronous: device publishes are persisted before the broker routes them.

## Requirements

- Go 1.21+
- PostgreSQL 14+

## Useful commands

```powershell
go mod tidy
go test ./...
go run ./cmd/mqtter-migrate
go run ./cmd/mqtter-server
cd frontend
npm install
npm run dev
```

Key environment variables:

- `MQTTER_DATABASE_URL`
- `MQTTER_HTTP_ADDR` defaults to `:8080`
- `MQTTER_MQTT_ADDR` defaults to `:1883`
- `MQTTER_BROKER_STORE_PATH` defaults to `data/broker-pebble`
- `MQTTER_BOOTSTRAP_ADMIN_USERNAME`
- `MQTTER_BOOTSTRAP_ADMIN_PASSWORD`
