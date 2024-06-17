package clients

import (
	"context"
	"fmt"
	"home_automation/internal/logger"
	"os"
	"time"

	"github.com/prometheus/client_golang/api"
	v1 "github.com/prometheus/client_golang/api/prometheus/v1"
	"github.com/prometheus/common/model"
)

type PromClient struct {
	client  v1.API
	address string
}

func InitPromClient() PromClient {
	address := "http://192.168.1.137:9090"
	client, err := api.NewClient(api.Config{
		Address: address,
	})
	if err != nil {
		fmt.Printf("Error creating client: %v\n", err)
		os.Exit(1)
	}

	v1api := v1.NewAPI(client)

	return PromClient{client: v1api, address: address}
}

func (promClient *PromClient) Query(metric string) ([]float64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, warnings, err := promClient.client.Query(ctx, metric, time.Now(), v1.WithTimeout(5*time.Second))

	if err != nil {
		logger.Error("Error querying Prometheus: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		logger.Warning("Warnings quering prometheus: %v", warnings)
	}

	logger.Trace("Value received from prometheus %v", result)

	values := []float64{}
	switch result.Type() {
	case model.ValScalar:
		scalarValue := result.(*model.Scalar)
		values[0] = float64(scalarValue.Value)
	case model.ValVector:
		vectorValue := result.(model.Vector)
		for _, elem := range vectorValue {
			values = append(values, float64(elem.Value))
		}
	default:
		logger.Warning("Unexpected value type for prometheus reponse: %s", result.Type())
	}

	return values, nil
}
