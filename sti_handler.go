package main

import (
	"context"
	"encoding/json"
	"errors"
	influxdb_utils "ias_sti/db/influxdb"

	ias_pg "ias_sti/db/pg"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	redis_lib "github.com/redis/go-redis/v9"

	"github.com/influxdata/influxdb-client-go/v2/api"
)

var (
	cacheRebuildMu  sync.Mutex
	cacheRebuilding bool
)

func getAllTreeSensorHandler(w http.ResponseWriter, r *http.Request, rdb *redis_lib.Client) {
	slog.Debug("Inbound HTTP", "process", "net.http", "application", "ias_sti", "endpoint", "/GET_ALL_TREE_SENSOR", "method", r.Method, "remote_addr", r.RemoteAddr)
	// Set content type header to JSON
	w.Header().Set("Content-Type", "application/json")

	// Send the response
	w.Write(getAllTreeSensor(rdb, r.Context()))
}

func getAllTreeSensor(rdb *redis_lib.Client, c context.Context) []byte {
	// Get All Tree Sensor Data
	// Check Redis Cache First
	cacheKey := "all_tree_sensors"
	cachedData, err := rdb.Get(c, cacheKey).Result()
	if err == nil && cachedData != "null" {
		// Cache hit, return cached data
		return_val := []byte(cachedData)
		return return_val
	}

	cacheRebuildMu.Lock()
	rebuilding := cacheRebuilding
	cacheRebuildMu.Unlock()
	if rebuilding {
		return []byte(`{"error":"cache is building, please try again"}`)
	}

	// Get sensors data if cache miss
	pg_storage := ias_pg.NewPostgresStorage(nil)
	sensors, err := pg_storage.QueryData("select * from ppj_tree_sensor")
	if err != nil {
		print(err)
	}

	// Encode sensors to JSON for both caching and response
	jsonData, err := json.Marshal(sensors)
	if err != nil {
		print(err)
	}
	// Write to cache with expiration (e.g., 5 minutes)
	cache_invalidation_seconds, err := strconv.Atoi(os.Getenv("REDIS_INVALIDATION_SECOND"))
	if err != nil {
		cache_invalidation_seconds = 60 // Default to 5 minutes if env variable is not set or invalid
	}
	cacheExpiration := time.Duration(cache_invalidation_seconds) * time.Second
	err = rdb.Set(c, cacheKey, jsonData, cacheExpiration).Err()
	if err != nil {
		// Log cache write error but don't fail the request
		// You might want to log this: log.Printf("Failed to write to cache: %v", err)
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
		err := rdb.Set(c, prefix+row["dev_eui"].(string), marshelled_json, 60*time.Second).Err()
		// cachedData, _ := rdb.Get(c, "battery:"+row["dev_eui"].(string)).Result()
		if err != nil {
			println("Error writing to cache:", err.Error())
		}
		tableCounter++
	}
}

func BuildSTICache(rdb *redis_lib.Client) bool {
	cacheRebuildMu.Lock()
	if cacheRebuilding {
		cacheRebuildMu.Unlock()
		return false
	}
	cacheRebuilding = true
	cacheRebuildMu.Unlock()

	defer func() {
		cacheRebuildMu.Lock()
		cacheRebuilding = false
		cacheRebuildMu.Unlock()
	}()

	// Get All Tree Sensor Data
	var sensor_array []ias_pg.PpjTreeSensor
	c := context.Background()
	sensors := getAllTreeSensor(rdb, c)
	json.Unmarshal(sensors, &sensor_array)

	query := buildFluxQueryForTreeSensorBattery(sensor_array)

	influx_result, err := influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "battery:")
	}

	query = buildFluxQueryForTreeSensorAngles(sensor_array)

	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "angle:")
	}

	query = buildFluxQueryForTreeSensorMagnitudeMax(sensor_array)

	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "magnitude_max:")
	}

	query = buildFluxQueryForTreeSensorMagnitudeMin(sensor_array)

	influx_result, err = influxdb_utils.RunQuery(query)
	if err == nil {
		createCacheFromInfluxDB(rdb, influx_result, c, "magnitude_min:")
	}

	return true
}

func getTreeSensorBatteryFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	// Check Redis Cache First
	cachedData, err := rdb.Get(c, "battery:"+devEUI).Result()
	if err == nil {
		return []byte(cachedData), nil
	}

	// If cache miss, trigger cache re-build
	if !BuildSTICache(rdb) {
		return nil, errors.New("cache is building, please try again")
	}

	// Try to get from cache again after rebuild
	cachedData, err = rdb.Get(c, "battery:"+devEUI).Result()
	if err != nil {
		return nil, err
	}
	return []byte(cachedData), nil

}

func getTreeSensorAngleFromCache(rdb *redis_lib.Client, c context.Context, devEUI string) ([]byte, error) {
	// Check Redis Cache First
	cachedData, err := rdb.Get(c, "angle:"+devEUI).Result()
	if err == nil {
		return []byte(cachedData), nil
	}

	// If cache miss, trigger cache re-build
	if !BuildSTICache(rdb) {
		return nil, errors.New("cache is building, please try again")
	}

	// Try to get from cache again after rebuild
	cachedData, err = rdb.Get(c, "angle:"+devEUI).Result()
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
