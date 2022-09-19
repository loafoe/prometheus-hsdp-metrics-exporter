package hsdp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/philips-software/go-hsdp-api/console"
	"github.com/prometheus/client_golang/prometheus"
)

type Metric struct {
	prometheus.Collector
	Updater
	name    string
	help    string
	query   string
	service string
	client  *console.Client
}

type OptionFunc func(m *Metric) error

type metricLabels struct {
	BrokerId             string `json:"broker_id"`
	DBInstanceIdentifier string `json:"dbinstance_identifier"`
	ExportedJob          string `json:"exported_job"`
	HsdpInstanceGuid     string `json:"hsdp_instance_guid"`
	HsdpInstanceName     string `json:"hsdp_instance_name"`
	Instance             string `json:"instance"`
	Job                  string `json:"job"`
	SpaceId              string `json:"space_id"`
	Service              string `json:"service,omitempty"`
}

func floatValue(input string) (fval float64) {
	fval, _ = strconv.ParseFloat(input, 64)
	return
}

type Updater interface {
	WithLabelValues(lvs ...string) prometheus.Gauge
}

func (metrics *Metric) Update(ctx context.Context, instance console.Instance) error {
	now := time.Now().Unix()
	// CPU
	dataResponse, _, err := metrics.client.Metrics.PrometheusGetData(ctx, instance.Details.Hostname,
		metrics.query,
		console.WithStart(now),
		console.WithEnd(now),
		console.WithStep(14))
	if err != nil {
		return err
	}
	for _, r := range dataResponse.Data.Result {
		var m metricLabels
		err := json.Unmarshal(r.Metric, &m)
		if err != nil {
			return err
		}
		if len(r.Values) == 0 {
			continue
		}
		val := r.Values[0]
		if len(val) < 2 {
			return fmt.Errorf("too few values received")
		}
		when := val[0].(float64)
		value := val[1].(string)
		fmt.Printf("  Metric: %+v\n", m)
		fmt.Printf("  Values: %f,%s\n", when, value)
		metrics.WithLabelValues(
			m.BrokerId,
			m.DBInstanceIdentifier,
			m.HsdpInstanceGuid,
			m.HsdpInstanceName,
			m.SpaceId,
			metrics.service,
		).Set(floatValue(value))
	}
	return nil
}

func NewMetric(opts ...OptionFunc) (*Metric, error) {
	m := &Metric{}

	for _, o := range opts {
		if err := o(m); err != nil {
			return nil, err
		}
	}

	gaugeVec := prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricNamePrefix + m.name,
		Help: m.help,
	}, []string{
		"broker_id",
		"dbinstance_identifier",
		"hsdp_instance_guid",
		"hsdp_instance_name",
		"space_id",
		"service",
	})
	m.Collector = gaugeVec
	m.Updater = gaugeVec

	return m, nil
}
