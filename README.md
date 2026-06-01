# ias_sti — Smart Tree Inventory

An IoT component that monitors tree tilt status to detect and alert when a tree has fallen. Part of the IAS ecosystem by Camart Sdn. Bhd.

## Architecture

```
Client → Go HTTP Server → Redis (cache)
              │
              ├── PostgreSQL (sensor metadata)
              └── InfluxDB (time-series sensor readings)
```

| Layer | Technology | Purpose |
|-------|-----------|---------|
| HTTP API | Go `net/http` | Serves sensor data over REST |
| Cache | Redis | Caches InfluxDB results with configurable TTL |
| Metadata | PostgreSQL (`pgx/v5`) | Stores sensor device info, locations, and configuration |
| Time-series | InfluxDB (`influxdb-client-go/v2`) | Stores raw sensor readings (angle, battery, magnitude) |

## Prerequisites

- **Go** 1.26+
- **PostgreSQL** with a `ppj_tree_sensor` table
- **InfluxDB** with a `STI` bucket populated with sensor data
- **Redis**
- **Docker** & **Docker Compose** (optional, for containerized deployment)

## Quick Start

### Local Development

```bash
cp .env.example .env
# Edit .env with your database credentials
go run .
```

The server listens on the port specified in `HTTP_SERVER_PORT` (default: `8081`).

### Docker Deployment

See [Build & Deploy](#build--deploy) for containerized deployment with Docker Compose.

## Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `HTTP_SERVER_PORT` | Port for the HTTP server | `8081` |
| `HTTP_SERVER_AUTOSTART` | Auto-start the server | `true` |
| `POSTGRES_HOST` | PostgreSQL host | `localhost` |
| `POSTGRES_PORT` | PostgreSQL port | `5432` |
| `POSTGRES_USER` | PostgreSQL user | `postgres` |
| `POSTGRES_PASSWORD` | PostgreSQL password | — |
| `POSTGRES_DB` | PostgreSQL database name | `ias_db` |
| `REDIS_HOST` | Redis host | `localhost` |
| `REDIS_PORT` | Redis port | `6379` |
| `REDIS_PASSWORD` | Redis password | — |
| `REDIS_INVALIDATION_SECOND` | Cache TTL in seconds | `60` |
| `INFLUXDB_URL` | InfluxDB URL | — |
| `INFLUXDB_TOKEN` | InfluxDB authentication token | — |
| `INFLUXDB_ORG` | InfluxDB organisation | — |
| `STI_AUTOMATION_ENABLE` | Pre-warm cache on startup (`true`/`false`) | `true` |
| `IAS_HC_BACKEND_ENABLE` | Enable Health Check backend integration | `true` |

## Build & Deploy

### Makefile Targets

| Target | Description |
|--------|-------------|
| `make build` | Build Linux amd64 binary (alias for `build-linux`) |
| `make build-linux` | Cross-compile for Linux amd64 (`CGO_ENABLED=0 GOOS=linux GOARCH=amd64`) |
| `make build-mac` | Build for macOS (native arch) |
| `make docker-build` | Build Linux binary, then Docker image (`ias_sti:latest`) |
| `make docker-run` | Run container standalone with `.env` |
| `make compose-up` | Start all services via `docker compose up -d` |
| `make compose-down` | Stop all services via `docker compose down` |
| `make clean` | Remove compiled binary |

### Docker Compose (Linux x86)

This guide walks through building and running the full stack on a Linux x86 host.

#### 1. Install Docker & Docker Compose

```bash
curl -fsSL https://get.docker.com | sh
sudo usermod -aG docker $USER
# Log out and back in for group changes to take effect

# Docker Compose plugin (Debian/Ubuntu)
sudo apt install docker-compose-plugin

# Docker Compose plugin (RHEL/CentOS/Fedora)
sudo yum install docker-compose-plugin
```

#### 2. Configure Environment

```bash
cp .env.example .env
# Edit .env with your database credentials
```

> When using docker-compose, set `REDIS_HOST=redis` (the service name in `docker-compose.yml`). The Redis container is internal-only — not exposed to the host.

#### 3. Build & Start

```bash
make docker-build   # compile Linux binary + build Docker image
make compose-up     # start Redis, API, and Dozzle
```

#### 4. Verify

```bash
curl http://localhost:8080/GET_ALL_TREE_SENSOR
docker compose logs -f   # tail all logs in real time
```

> Dozzle log viewer is available at `http://<host>:8888`

#### 5. Stop

```bash
make compose-down
```

### Compose Services

| Service | Image | Port (host) | Notes |
|---------|-------|-------------|-------|
| `api` | `ias_sti:latest` | `8080` | Your application |
| `redis` | `redis:7-alpine` | internal only | In-memory cache with AOF persistence |
| `dozzle` | `amir20/dozzle:latest` | `8888` | Web-based Docker log viewer |

## API Reference

All responses are JSON (`Content-Type: application/json`).

### `GET /GET_ALL_TREE_SENSOR`

Returns metadata for all sensors registered in the database.

**Response** — JSON array of sensor objects:

```json
[
  {
    "device_eui": "A84041...",
    "device_name": "Tree Sensor 01",
    "latitude": 3.1569,
    "longitude": 101.7123,
    "displacement_alert_angle": 15,
    "note": "Near main road",
    "tree_id": "TR-001",
    "id": 1,
    "gateway_id": "GW-01",
    "gateway_name": "Gateway KLCC",
    "description": "Lorawan gateway",
    "gateway_eui": "B827EB...",
    "network_server": "ChirpStack",
    "gateway_model": "LPS8",
    "last_seen": "2025-01-01T00:00:00Z",
    "is_active": true,
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2025-01-01T00:00:00Z"
  }
]
```

**Error** — if cache is rebuilding:
```json
{"error":"cache is building, please try again"}
```

### `GET /GET_TREE_SENSOR_BATTERY?dev_eui=<eui>`

Latest battery reading for a specific sensor.

```json
{
  "result": "_result",
  "table": 0,
  "_start": "2025-01-01T00:00:00Z",
  "_stop": "2025-01-02T00:00:00Z",
  "_time": "2025-01-01T12:00:00Z",
  "_value": 95.5,
  "_field": "value",
  "_measurement": "device_frmpayload_data_battery",
  "dev_eui": "A84041..."
}
```

### `GET /GET_TREE_SENSOR_ANGLE?dev_eui=<eui>`

Latest x/y/z angle readings (last 30 days), pivoted.

```json
{
  "result": "_result",
  "table": 0,
  "_start": "2025-01-01T00:00:00Z",
  "_stop": "2025-01-02T00:00:00Z",
  "_time": "2025-01-01T12:00:00Z",
  "_value": 2.3,
  "_field": "value",
  "_measurement": "device_frmpayload_data_angle_x",
  "device_frmpayload_data_angle_y": 1.1,
  "device_frmpayload_data_angle_z": 89.5,
  "dev_eui": "A84041..."
}
```

### `GET /GET_TREE_SENSOR_MAGNITUDE_MIN?dev_eui=<eui>`

Minimum magnitude reading in the last 24 hours.

```json
{
  "result": "_result",
  "table": 0,
  "_start": "2025-01-01T00:00:00Z",
  "_stop": "2025-01-02T00:00:00Z",
  "_time": "2025-01-01T12:00:00Z",
  "_value": 0.12,
  "_field": "value",
  "_measurement": "device_frmpayload_data_magnitude",
  "dev_eui": "A84041..."
}
```

### `GET /GET_TREE_SENSOR_MAGNITUDE_MAX?dev_eui=<eui>`

Maximum magnitude reading in the last 24 hours.

```json
{
  "result": "_result",
  "table": 0,
  "_start": "2025-01-01T00:00:00Z",
  "_stop": "2025-01-02T00:00:00Z",
  "_time": "2025-01-01T12:00:00Z",
  "_value": 7.85,
  "_field": "value",
  "_measurement": "device_frmpayload_data_magnitude",
  "dev_eui": "A84041..."
}
```

## Caching Behaviour

- On startup, if `STI_AUTOMATION_ENABLE=true`, the service pre-warms the Redis cache by querying PostgreSQL → InfluxDB for all sensors.
- Battery and angle endpoints trigger an on-demand cache rebuild on cache miss.
- Cache rebuilds are mutex-guarded to prevent concurrent redundant work.
- Magnitude min/max endpoints are read-only — they do not trigger rebuilds and return `null` on cache miss.
- Cache TTL is controlled by `REDIS_INVALIDATION_SECOND`.

## License

GNU Affero General Public License v3.0 (AGPL-3.0)

## Author

haziqnorisham — Camart Sdn. Bhd.
