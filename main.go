package main

import (
	"context"
	influxdb_utils "ias_sti/db/influxdb"
	ias_pg "ias_sti/db/pg"
	redis_utils "ias_sti/db/redis"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	redis_lib "github.com/redis/go-redis/v9"
)

var IsRunning = false
var currentServer *http.Server
var cacheWatchdogCancel context.CancelFunc

func main() {
	initLogger()
	loadEnv()
	initSharedPool()
	defer ias_pg.CloseSharedPool()
	rdb := initRedis()
	defer rdb.Close()
	initInfluxDB()
	startCacheWatchdog(rdb)
	registerRoutes(rdb)
	startServer()
	waitForShutdown(rdb)
}

func initLogger() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))
	slog.SetDefault(logger)
	slog.Info("STI Service is starting")
}

func loadEnv() {
	if err := godotenv.Load(".env"); err != nil {
		slog.Error("Failed to load .env file", "error", err)
		os.Exit(1)
	}
	slog.Info("Environment variables loaded", "process", "sti_main")
}

func initSharedPool() {
	slog.Info("Initializing PostgreSQL shared connection pool", "process", "sti_main")
	if err := ias_pg.InitSharedPool(); err != nil {
		slog.Error("Failed to initialize PostgreSQL shared pool", "error", err)
		os.Exit(1)
	}
	slog.Info("PostgreSQL shared connection pool initialized", "process", "sti_main")
}

func initRedis() *redis_lib.Client {
	slog.Info("Initializing Redis connection", "process", "sti_main")
	rdb := redis_utils.NewRedisClient()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("Redis ping failed", "error", err)
		os.Exit(1)
	}
	slog.Info("Redis connection established", "process", "sti_main")
	return rdb
}

func initInfluxDB() {
	slog.Info("Initializing InfluxDB connection", "process", "sti_main")
	if err := influxdb_utils.InitInfluxService(os.Getenv("INFLUXDB_ORG")); err != nil {
		slog.Error("Failed to initialize InfluxDB", "error", err)
		os.Exit(1)
	}
	slog.Info("InfluxDB connection established", "process", "sti_main")
}

func startCacheWatchdog(rdb *redis_lib.Client) {
	refreshSeconds, err := strconv.Atoi(os.Getenv("STI_CACHE_REFRESH_SECONDS"))
	if err != nil || refreshSeconds <= 0 {
		slog.Warn("STI_CACHE_REFRESH_SECONDS not set or invalid, cache watchdog disabled", "value", os.Getenv("STI_CACHE_REFRESH_SECONDS"))
		return
	}
	interval := time.Duration(refreshSeconds) * time.Second

	slog.Info("Building initial STI cache", "process", "sti_main")
	BuildSTICache(rdb)

	ctx, cancel := context.WithCancel(context.Background())
	cacheWatchdogCancel = cancel

	go func() {
		slog.Info("STI cache watchdog started", "interval_seconds", refreshSeconds, "process", "sti_main")
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				slog.Info("STI cache watchdog stopped", "process", "sti_main")
				return
			case <-ticker.C:
				BuildSTICache(rdb)
			}
		}
	}()
}

func registerRoutes(rdb *redis_lib.Client) {
	http.HandleFunc("/GET_ALL_TREE_SENSOR", func(w http.ResponseWriter, r *http.Request) {
		getAllTreeSensorHandler(w, r, rdb)
	})
	http.HandleFunc("/GET_TREE_SENSOR_BATTERY", func(w http.ResponseWriter, r *http.Request) {
		getTreeSensorBatteryHandler(w, r, rdb)
	})
	http.HandleFunc("/GET_TREE_SENSOR_ANGLE", func(w http.ResponseWriter, r *http.Request) {
		getTreeSensorAngleHandler(w, r, rdb)
	})
	http.HandleFunc("/GET_TREE_SENSOR_MAGNITUDE_MIN", func(w http.ResponseWriter, r *http.Request) {
		getTreeSensorMagnitudeMinHandler(w, r, rdb)
	})
	http.HandleFunc("/GET_TREE_SENSOR_MAGNITUDE_MAX", func(w http.ResponseWriter, r *http.Request) {
		getTreeSensorMagnitudeMaxHandler(w, r, rdb)
	})
}

func startServer() {
	currentServer = &http.Server{Addr: ":" + os.Getenv("HTTP_SERVER_PORT")}
	go func() {
		IsRunning = true
		slog.Info("STI HTTP server started on "+currentServer.Addr, "address", currentServer.Addr, "process", "sti_main")
		if err := currentServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
		}
	}()
	time.Sleep(100 * time.Millisecond)
}

func waitForShutdown(rdb *redis_lib.Client) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("Received signal, shutting down STI service", "signal", sig.String())

	if cacheWatchdogCancel != nil {
		cacheWatchdogCancel()
	}

	if IsRunning {
		slog.Info("Shutting down HTTP server", "process", "sti_main")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := currentServer.Shutdown(ctx); err != nil {
			slog.Error("HTTP server forced shutdown", "error", err)
		}
		IsRunning = false
		slog.Info("HTTP server stopped gracefully", "process", "sti_main")
	}

	slog.Info("STI service shut down gracefully", "process", "sti_main")
}
