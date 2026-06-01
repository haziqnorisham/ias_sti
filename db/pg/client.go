package pg

import (
	"database/sql"
	"log/slog"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
)

var sharedPool *sql.DB

func InitSharedPool() error {
	dsn := "postgres://" + os.Getenv("POSTGRES_USER") + ":" + os.Getenv("POSTGRES_PASSWORD") + "@" + os.Getenv("POSTGRES_HOST") + ":" + os.Getenv("POSTGRES_PORT") + "/" + os.Getenv("POSTGRES_DB") + "?sslmode=disable"
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return err
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return err
	}
	sharedPool = db
	slog.Info("PostgreSQL shared connection pool initialized", "process", "pg_client")
	return nil
}

func CloseSharedPool() {
	if sharedPool != nil {
		sharedPool.Close()
		slog.Info("PostgreSQL shared connection pool closed", "process", "pg_client")
	}
}

type PostgresStorage struct {
	DB *sql.DB
}

type PpjTreeSensor struct {
	DeviceEUI              string  `db:"device_eui"`
	DeviceName             string  `db:"device_name"`
	Latitude               float64 `db:"latitude"`
	Longitude              float64 `db:"longitude"`
	DisplacementAlertAngle *int32  `db:"displacement_alert_angle"`
	Note                   *string `db:"note"`
	TreeID                 *string `db:"tree_id"`
	ID                     *int32  `db:"id"`
	GatewayID              *string `db:"gateway_id"`
	GatewayName            *string `db:"gateway_name"`
	Description            *string `db:"description"`
	GatewayEUI             *string `db:"gateway_eui"`
	NetworkServer          *string `db:"network_server"`
	GatewayModel           *string `db:"gateway_model"`
	LastSeen               *string `db:"last_seen"`
	IsActive               *bool   `db:"is_active"`
	CreatedAt              *string `db:"created_at"`
	UpdatedAt              *string `db:"updated_at"`
}

type EnrichedTreeSensor struct {
	PpjTreeSensor
	BatteryLevel  *float64 `json:"battery_level"`
	AngleX        *float64 `json:"x"`
	AngleY        *float64 `json:"y"`
	AngleZ        *float64 `json:"z"`
	AngleTime     *string  `json:"time"`
	TiltMagnitude *float64 `json:"tilt_magnitude"`
	MagnitudeMin  *float64 `json:"magnitude_min"`
	MagnitudeMax  *float64 `json:"magnitude_max"`
	DiffMag       *float64 `json:"diff_mag"`
	IsDanger      bool     `json:"is_danger"`
	TreeIDLower   *string  `json:"tree_id"`
	TreeIDLink    string   `json:"tree_id_link"`
	NormalCount   int      `json:"normal_count"`
	DangerCount   int      `json:"danger_count"`
}

func NewPostgresStorage(db *sql.DB) *PostgresStorage {
	if db != nil {
		return &PostgresStorage{DB: db}
	}
	return &PostgresStorage{DB: sharedPool}
}

func (p *PostgresStorage) QueryData(query string) ([]PpjTreeSensor, error) {
	rows, err := p.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sensors []PpjTreeSensor

	for rows.Next() {
		var sensor PpjTreeSensor
		err := rows.Scan(
			&sensor.DeviceEUI,
			&sensor.DeviceName,
			&sensor.Latitude,
			&sensor.Longitude,
			&sensor.DisplacementAlertAngle,
			&sensor.Note,
			&sensor.TreeID,
			&sensor.ID,
			&sensor.GatewayID,
			&sensor.GatewayName,
			&sensor.Description,
			&sensor.GatewayEUI,
			&sensor.NetworkServer,
			&sensor.GatewayModel,
			&sensor.LastSeen,
			&sensor.IsActive,
			&sensor.CreatedAt,
			&sensor.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		sensors = append(sensors, sensor)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return sensors, nil
}
