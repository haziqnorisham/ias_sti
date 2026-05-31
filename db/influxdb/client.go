package influxdb

import (
	"context"
	"fmt"
	"log"
	"os"

	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
)

type InfluxDBService struct {
	client influxdb2.Client
	org    string
}

// Constructor
func NewInfluxDBService(org string) (*InfluxDBService, error) {
	token := os.Getenv("INFLUXDB_TOKEN")
	url := os.Getenv("INFLUXDB_URL")

	client := influxdb2.NewClient(url, token)

	return &InfluxDBService{
		client: client,
		org:    org,
	}, nil
}

func (s *InfluxDBService) Query(ctx context.Context, query string) (*api.QueryTableResult, error) {
	queryAPI := s.client.QueryAPI(s.org)
	results, err := queryAPI.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	return results, results.Err()
}

func (s *InfluxDBService) Close() {
	if s.client != nil {
		s.client.Close()
	}
}

// In your main application
var globalInfluxService *InfluxDBService

func InitInfluxService(org string) error {
	var err error
	globalInfluxService, err = NewInfluxDBService(org)
	return err
}

func GetInfluxService() *InfluxDBService {
	if globalInfluxService == nil {
		log.Fatal("InfluxDB service not initialized")
	}
	return globalInfluxService
}

func TestQuery() {
	if globalInfluxService == nil {
		log.Fatal("InfluxDB service not initialized")
	}

	query := `from(bucket: "STI")
  |> range(start: 0, stop: now())
  |> filter(fn: (r) => r["dev_eui"] == "")
  |> filter(fn: (r) => r["_measurement"] == "device_frmpayload_data_battery")
  |> last()`

	results, err := globalInfluxService.Query(context.Background(), query)
	if err != nil {
		log.Fatal(err)
	}
	if results.Next() {
		record := results.Record()
		fmt.Printf("Value: %v\n", record.Value())
		fmt.Printf("Time: %v\n", record.Time())
		fmt.Println("Measurement:", record.Measurement())
		fmt.Println("Longitude:", record.ValueByKey("longitude"))
	}
	results.Close()
}

func RunQuery(query string) (*api.QueryTableResult, error) {
	if globalInfluxService == nil {
		log.Fatal("InfluxDB service not initialized")
	}
	return globalInfluxService.Query(context.Background(), query)
}
