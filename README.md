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

## Quick Start

```bash
cp .env.example .env
# Edit .env with your database credentials
go run .
```

The server listens on the port specified in `HTTP_SERVER_PORT` (default: `8081`).

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
