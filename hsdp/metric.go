package hsdp

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/patrickmn/go-cache"
	"github.com/philips-software/go-hsdp-api/console"
	"github.com/prometheus/client_golang/prometheus"
)

type Metric struct {
	cache *cache.Cache
	prometheus.Collector
	metricVec *prometheus.MetricVec
	Updater
	name    string
	help    string
	query   string
	service string
	region  string
	prune   int
	client  *console.Client
}

type OptionFunc func(m *Metric) error

type metricLabels struct {
	BrokerId             string `json:"broker_id"`
	DBInstanceIdentifier string `json:"dbinstance_identifier"`
	ExportedJob          string `json:"exported_job"`
	HsdpInstanceGuid     string `json:"hsdp_instance_guid"`
	HsdpInstanceName     string `json:"hsdp_instance_name"`
	HsdpServiceName      string `json:"hsdp_service_name,omitempty"`
	HsdpInstanceNodeName string `json:"hsdp_instance_node_name,omitempty"`
	Instance             string `json:"instance"`
	Job                  string `json:"job"`
	SpaceId              string `json:"space_id"`
	Service              string `json:"service,omitempty"`
}

type cacheItem struct {
	labels     metricLabels
	lastUpdate time.Time
}

func (m *metricLabels) Label() string {
	return "hsdp_instance_guid"
}

func (m *metricLabels) Identifier() string {
	return m.HsdpInstanceGuid
}

func floatValue(input string) (fVal float64) {
	fVal, _ = strconv.ParseFloat(input, 64)
	return
}

type Updater interface {
	WithLabelValues(lvs ...string) prometheus.Gauge
}

func (metrics *Metric) Prune() {
	// Clean up
	items := metrics.cache.Items()
	for key, entry := range items {
		item, ok := entry.Object.(cacheItem)
		if !ok {
			continue
		}
		prune := time.Duration(time.Duration(metrics.prune) * time.Second).Seconds()
		if stale := time.Since(item.lastUpdate).Seconds(); stale > prune { // Prune
			deleted := metrics.metricVec.DeletePartialMatch(map[string]string{
				item.labels.Label(): item.labels.Identifier(),
			})
			fmt.Printf("Deleted %d instance(s): %s:%s, metric: %s (%f > %f)\n", deleted, item.labels.Label(), item.labels.Identifier(), metrics.name, stale, prune)
			// Remove from cache
			metrics.cache.Delete(key)
		}
	}
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
		_ = val[0].(float64)
		value := val[1].(string)
		//fmt.Printf("  Metric: %+v\n", m)
		//fmt.Printf("  Values: %f,%s\n", when, value)
		metrics.WithLabelValues(
			m.BrokerId,
			m.DBInstanceIdentifier,
			m.HsdpInstanceGuid,
			m.HsdpInstanceName,
			m.SpaceId,
			metrics.service,
			metrics.region,
		).Set(floatValue(value))
		// Update cache
		_, stored := metrics.cache.Get(m.Identifier())
		if !stored {
			fmt.Printf("Adding new instance %s %s\n", m.Identifier(), metrics.name)
		}
		metrics.cache.Set(m.Identifier(), cacheItem{
			labels:     m,
			lastUpdate: time.Now(),
		}, -1)
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
		"region",
	})
	m.Collector = gaugeVec
	m.Updater = gaugeVec
	m.metricVec = gaugeVec.MetricVec
	m.cache = cache.New(720*time.Minute, 1440*time.Minute)

	// Start pruner
	go func() {
		tick := time.Tick(time.Duration(m.prune) * time.Second)
		for range tick {
			m.Prune() // Clear any stale metrics first
		}
	}()

	return m, nil
}
