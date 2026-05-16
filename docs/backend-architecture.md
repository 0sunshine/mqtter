# Backend Architecture

## Runtime

```mermaid
flowchart LR
  Dev["MQTT Devices"] --> Broker["Mochi MQTT Broker"]
  Broker --> Hook["Thin Hooks"]
  Hook --> Ingest["Synchronous Ingestion"]
  Ingest --> PG["PostgreSQL"]
  Broker --> Pebble["Pebble Broker Store"]

  UI["Vue Admin"] --> API["REST API"]
  UI --> WS["WebSocket"]
  API --> Svc["Application Services"]
  Svc --> PG
  Svc --> Pub["MQTT Publisher Port"]
  Pub --> Broker
  Ingest --> Alert["Alert Service"]
  Alert --> WS
```

## Modules

```mermaid
flowchart TD
  cmd["cmd/mqtter-server"] --> app["internal/app"]
  app --> broker["internal/broker"]
  app --> api["internal/api"]
  app --> domain["internal/domain"]
  app --> realtime["internal/realtime"]
  app --> jobs["internal/jobs"]
  domain --> ports["internal/ports"]
  ports --> pg["internal/storage/postgres"]
  broker --> pebble["data/broker-pebble"]
```

## API

All management routes except `POST /api/auth/login` require the HttpOnly session cookie.

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/auth/login` | Login and set the session cookie |
| `POST` | `/api/auth/logout` | Delete the current session |
| `GET` | `/api/me` | Current admin user |
| `GET` | `/api/devices` | Device list with `status/type/q/page/pageSize` |
| `GET` | `/api/devices/{deviceId}` | Device detail |
| `PATCH` | `/api/devices/{deviceId}/type` | Change device type |
| `GET` | `/api/devices/{deviceId}/topics` | Observed topics for one device |
| `GET` | `/api/topics` | Global observed topics |
| `GET` | `/api/messages` | Message history; defaults to the last 24 hours |
| `POST` | `/api/publish` | Publish a text payload to a concrete topic |
| `GET` | `/api/commands` | Publish command audit list |
| `GET` | `/api/alerts` | System alerts, including overload alerts |
| `GET` | `/api/device-types` | Registered device types |
| `GET` | `/api/realtime` | WebSocket event stream |

## Testability

- HTTP handlers depend on small interfaces in `internal/api`.
- MQTT hooks depend only on `BrokerIngestor`.
- Services depend on repository and publisher ports from `internal/ports`.
- PostgreSQL code is isolated in `internal/storage/postgres`.
- Time, IDs, MQTT publish, password hashing, alerts, and realtime events are injected for tests.
