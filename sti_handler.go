package main

import (
	"context"
	"encoding/json"
	influxdb_utils "ias_sti/db/influxdb"

	ias_pg "ias_sti/db/pg"
	"log/slog"
	"net/http"
	"strings"

	redis_lib "github.com/redis/go-redis/v9"

	"github.com/influxdata/influxdb-client-go/v2/api"
)

func getAllTreeSensorHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_ALL_TREE_SENSOR", "method", r.Method, "remote_addr", r.RemoteAddr)
	// Set content type header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Send the response
	w.Write(getAllTreeSensor(rdb, r.Context()))
}

func getAllTreeSensor(rdb *redis_lib.Client, c context.Context) []byte {
	cacheKey := "all_tree_sensors"
	cachedData, err := rdb.Get(c, cacheKey).Result()
	if err == nil && cachedData != "null" {
		return []byte(cachedData)
	}

	pg_storage := ias_pg.NewPostgresStorage(nil)
	sensors, err := pg_storage.QueryData("select * from ppj_tree_sensor")
	if err != nil {
		slog.Error("getAllTreeSensor: PG query failed", "error", err)
		return []byte(`{"error":"failed to query database"}`)
	}

	jsonData, err := json.Marshal(sensors)
	if err != nil {
		slog.Error("getAllTreeSensor: marshal failed", "error", err)
		return []byte(`{"error":"failed to encode response"}`)
	}
	return jsonData
}

func buildFluxQueryForTreeSensorBattery(sensor_array []ias_pg.PpjTreeSensor) string {
	// Build the InfluxDB Query

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`from(bucket: "STI")
		|> range(start: 0, stop: now())
		|> filter(fn: (r) => r["_measurement"] == "device_frmpayload_data_battery")
		|> filter(fn: (r) => `)

	if len(sensor_array) > 0 {
		var conditions []string
		for _, sensor := range sensor_array {
			// Escape any quotes in the sensor value
			escapedSensor := strings.ReplaceAll(sensor.DeviceEUI, `"`, `\"`)
			conditions = append(conditions, `r["dev_eui"] == "`+escapedSensor+`"`)
		}
		queryBuilder.WriteString(strings.Join(conditions, " or "))
	} else {
		queryBuilder.WriteString(`r["dev_eui"] == ""`)
	}

	queryBuilder.WriteString(`)  |> group(columns: ["dev_eui"])|> last()`)

	return queryBuilder.String()
}

func buildFluxQueryForTreeSensorAngles(sensor_array []ias_pg.PpjTreeSensor) string {
	// Build the InfluxDB Query

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`from(bucket: "STI")
  |> range(start: -30d, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "device_frmpayload_data_angle_y" or r["_measurement"] == "device_frmpayload_data_angle_z" or r["_measurement"] == "device_frmpayload_data_angle_x")
  |> filter(fn: (r) => r["_field"] == "value")
  |> filter(fn: (r) => `)

	if len(sensor_array) > 0 {
		var conditions []string
		for _, sensor := range sensor_array {
			// Escape any quotes in the sensor value
			escapedSensor := strings.ReplaceAll(sensor.DeviceEUI, `"`, `\"`)
			conditions = append(conditions, `r["dev_eui"] == "`+escapedSensor+`"`)
		}
		queryBuilder.WriteString(strings.Join(conditions, " or "))
	} else {
		queryBuilder.WriteString(`r["dev_eui"] == ""`)
	}

	queryBuilder.WriteString(`)
  |> last()
  |> pivot(
    rowKey: ["_time", "dev_eui", "device_name", "application_name"],
    columnKey: ["_measurement"],
    valueColumn: "_value"
  )
`)

	return queryBuilder.String()
}

func buildFluxQueryForTreeSensorMagnitudeMax(sensor_array []ias_pg.PpjTreeSensor) string {
	// Build the InfluxDB Query

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`from(bucket: "STI")
  |> range(start: -24h, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "device_frmpayload_data_magnitude")
  |> filter(fn: (r) => `)

	if len(sensor_array) > 0 {
		var conditions []string
		for _, sensor := range sensor_array {
			// Escape any quotes in the sensor value
			escapedSensor := strings.ReplaceAll(sensor.DeviceEUI, `"`, `\"`)
			conditions = append(conditions, `r["dev_eui"] == "`+escapedSensor+`"`)
		}
		queryBuilder.WriteString(strings.Join(conditions, " or "))
	} else {
		queryBuilder.WriteString(`r["dev_eui"] == ""`)
	}

	queryBuilder.WriteString(`) |> max()`)

	return queryBuilder.String()
}

func buildFluxQueryForTreeSensorMagnitudeMin(sensor_array []ias_pg.PpjTreeSensor) string {
	// Build the InfluxDB Query

	queryBuilder := strings.Builder{}
	queryBuilder.WriteString(`from(bucket: "STI")
  |> range(start: -24h, stop: now())
  |> filter(fn: (r) => r["_measurement"] == "device_frmpayload_data_magnitude")
  |> filter(fn: (r) => `)

	if len(sensor_array) > 0 {
		var conditions []string
		for _, sensor := range sensor_array {
			// Escape any quotes in the sensor value
			escapedSensor := strings.ReplaceAll(sensor.DeviceEUI, `"`, `\"`)
			conditions = append(conditions, `r["dev_eui"] == "`+escapedSensor+`"`)
		}
		queryBuilder.WriteString(strings.Join(conditions, " or "))
	} else {
		queryBuilder.WriteString(`r["dev_eui"] == ""`)
	}

	queryBuilder.WriteString(`) |> min()`)

	return queryBuilder.String()
}
func createCacheFromInfluxDB(rdb *redis_lib.Client, influx_result *api.QueryTableResult, c context.Context, prefix string) {
	var results []map[string]interface{}
	tableCounter := 0

	for influx_result.Next() {
		record := influx_result.Record()

		// Build dynamic map
		row := map[string]interface{}{
			"result":       "_result",
			"table":        tableCounter,
			"_start":       record.Start(),
			"_stop":        record.Stop(),
			"_time":        record.Time(),
			"_value":       record.Value(),
			"_field":       record.Field(),
			"_measurement": record.Measurement(),
		}

		// Add all values from the record
		for key, value := range record.Values() {
			// Skip keys we already set
			if key != "_start" && key != "_stop" && key != "_time" &&
				key != "_value" && key != "_field" && key != "_measurement" {
				row[key] = value
			}
		}

		results = append(results, row)
		marshelled_json, _ := json.Marshal(row)
		err := rdb.Set(c, prefix+row["dev_eui"].(string), marshelled_json, 0).Err()
		// cachedData, _ := rdb.Get(c, "battery:"+row["dev_eui"].(string)).Result()
		if err != nil {
			println("Error writing to cache:", err.Error())
		}
		tableCounter++
	}
}

func BuildSTICache(rdb *redis_lib.Client) {
	c := context.Background()

	pg_storage := ias_pg.NewPostgresStorage(nil)
	sensors, err := pg_storage.QueryData("select * from ppj_tree_sensor")
	if err != nil {
		slog.Error("BuildSTICache: PG query failed", "error", err)
		return
	}

	query := buildFluxQueryForTreeSensorBattery(sensors)
	influx_result, err := influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "battery:")
	} else {
		slog.Error("BuildSTICache: battery query failed", "error", err)
	}

	query = buildFluxQueryForTreeSensorAngles(sensors)
	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "angle:")
	} else {
		slog.Error("BuildSTICache: angle query failed", "error", err)
	}

	query = buildFluxQueryForTreeSensorMagnitudeMax(sensors)
	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "magnitude_max:")
	} else {
		slog.Error("BuildSTICache: magnitude_max query failed", "error", err)
	}

	query = buildFluxQueryForTreeSensorMagnitudeMin(sensors)
	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "magnitude_min:")
	} else {
		slog.Error("BuildSTICache: magnitude_min query failed", "error", err)
	}

	enriched := make([]ias_pg.EnrichedTreeSensor, 0, len(sensors))
	for _, sensor := range sensors {
		e := ias_pg.EnrichedTreeSensor{PpjTreeSensor: sensor}
		batteryJSON, berr := rdb.Get(c, "battery:"+sensor.DeviceEUI).Result()
		if berr == nil {
			var batteryMap map[string]interface{}
			if json.Unmarshal([]byte(batteryJSON), &batteryMap) == nil {
				if v, ok := batteryMap["_value"].(float64); ok {
					e.BatteryLevel = &v
				}
			}
		}
		enriched = append(enriched, e)
	}

	jsonData, err := json.Marshal(enriched)
	if err != nil {
		slog.Error("BuildSTICache: failed to marshal enriched sensors", "error", err)
		return
	}

	err = rdb.Set(c, "all_tree_sensors", jsonData, 0).Err()
	if err != nil {
		slog.Error("BuildSTICache: failed to cache enriched sensors", "error", err)
	}
}

func getTreeSensorBatteryFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	cachedData, err := rdb.Get(c, "battery:"+devEUI).Result()
	if err != nil {
		return nil, err
	}
	return []byte(cachedData), nil
}

func getTreeSensorAngleFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	cachedData, err := rdb.Get(c, "angle:"+devEUI).Result()
	if err != nil {
		return nil, err
	}
	return []byte(cachedData), nil
}

func getTreeSensorMagnitudeMinFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	// Check Redis Cache First
	cachedData_min, err := rdb.Get(c, "magnitude_min:"+devEUI).Result()
	if err == nil {
		return []byte(cachedData_min), nil
	}
	return nil, err
}

func getTreeSensorMagnitudeMaxFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	// Check Redis Cache First
	cachedData_max, err := rdb.Get(c, "magnitude_max:"+devEUI).Result()
	if err == nil {
		return []byte(cachedData_max), nil
	}
	return nil, err
}

func getTreeSensorBatteryHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_TREE_SENSOR_BATTERY", "method", r.Method, "remote_addr", r.RemoteAddr)

	// Set content type header to JSON
	w.Header().Set("Content-Type", "application/json")
	devEUI := r.URL.Query().Get("dev_eui")
	// Send the response
	rerurn_val, err := getTreeSensorBatteryFromCache(rdb, r.Context(), devEUI)
	if err != nil {
		http.Error(w, "Error retrieving data", http.StatusInternalServerError)
		return
	}
	w.Write(rerurn_val)
}

func getTreeSensorAngleHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_TREE_SENSOR_ANGLE", "method", r.Method, "remote_addr", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	devEUI := r.URL.Query().Get("dev_eui")
	rerurn_val, err := getTreeSensorAngleFromCache(rdb, r.Context(), devEUI)
	if err != nil {
		http.Error(w, "Error retrieving data", http.StatusInternalServerError)
		return
	}
	w.Write(rerurn_val)
}

func getTreeSensorMagnitudeMinHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_TREE_SENSOR_MAGNITUDE_MIN", "method", r.Method, "remote_addr", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	devEUI := r.URL.Query().Get("dev_eui")
	rerurn_val, _ := getTreeSensorMagnitudeMinFromCache(rdb, r.Context(), devEUI)

	w.Write(rerurn_val)
}

func getTreeSensorMagnitudeMaxHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_TREE_SENSOR_MAGNITUDE_MAX", "method", r.Method, "remote_addr", r.RemoteAddr)

	w.Header().Set("Content-Type", "application/json")
	devEUI := r.URL.Query().Get("dev_eui")
	rerurn_val, _ := getTreeSensorMagnitudeMaxFromCache(rdb, r.Context(), devEUI)

	w.Write(rerurn_val)
}
