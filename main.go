package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/philips-software/go-hsdp-api/console"
)

var region string
var listenAddr string
var debugLog string
var metricNamePrefix = "hsdp_metric_"

var (
	registry = prometheus.NewRegistry()

	rdsCPUMetric = prometheus.NewGaugeVec(prometheus.GaugeOpts{
		Name: metricNamePrefix + "rds_cpu_average",
		Help: "HSDP RDS database CPU utilization average",
	}, []string{
		"broker_id",
		"dbinstance_identifier",
		"hsdp_instance_guid",
		"hsdp_instance_name",
		"space_id",
	})
)

func init() {
	registry.MustRegister(rdsCPUMetric)
}

type metric struct {
	BrokerId             string `json:"broker_id"`
	DBInstanceIdentifier string `json:"dbinstance_identifier"`
	ExportedJob          string `json:"exported_job"`
	HsdpInstanceGuid     string `json:"hsdp_instance_guid"`
	HsdpInstanceName     string `json:"hsdp_instance_name"`
	Instance             string `json:"instance"`
	Job                  string `json:"job"`
	SpaceId              string `json:"space_id"`
}

func floatValue(input string) (fval float64) {
	fval, _ = strconv.ParseFloat(input, 64)
	return
}

func main() {
	flag.StringVar(&debugLog, "debuglog", "", "The debug log to dump traffic in")
	flag.StringVar(&region, "region", "eu-west", "The HSDP region to use")
	flag.StringVar(&listenAddr, "listen", "0.0.0.0:8889", "Listen address for HTTP metrics")
	flag.Parse()

	uaaUsername := os.Getenv("UAA_USERNAME")
	uaaPassword := os.Getenv("UAA_PASSWORD")

	if uaaUsername == "" || uaaPassword == "" {
		fmt.Printf("missing UAA_USERNAME or UAA_PASSWORD environment values\n")
		os.Exit(1)
	}

	uaaClient, err := console.NewClient(nil, &console.Config{
		Region:   region,
		DebugLog: debugLog,
	})
	if err != nil {
		fmt.Printf("Error setting up console client: %v\n", err)
		return
	}
	err = uaaClient.Login(uaaUsername, uaaPassword)
	if err != nil {
		fmt.Printf("Error logging into UAA: %v\n", err)
		return
	}
	ctx := context.Background()

	go func() {
		sleep := false
		for {
			if sleep {
				time.Sleep(time.Second * 15)
			}
			sleep = true
			instances, err := uaaClient.Metrics.GQLGetInstances(ctx)
			if err != nil {
				continue
			}
			for _, instance := range *instances {
				fmt.Printf("Instance: %+v\n", instance.GUID)
				now := time.Now().Unix()
				data, _, err := uaaClient.Metrics.PrometheusGetData(ctx, instance.Details.Hostname,
					"(aws_rds_cpuutilization_average) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})",
					console.WithStart(now),
					console.WithEnd(now),
					console.WithStep(14))
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					continue
				}
				for _, r := range data.Data.Result {
					var m metric
					err := json.Unmarshal(r.Metric, &m)
					if err != nil {
						continue
					}
					if len(r.Values) == 0 {
						continue
					}
					val := r.Values[0]
					if len(val) < 2 {
						continue
					}
					when := val[0].(float64)
					value := val[1].(string)
					fmt.Printf("  Metrics: %+v\n", m)
					fmt.Printf("  Values: %f,%s\n", when, value)
					rdsCPUMetric.WithLabelValues(
						m.BrokerId,
						m.DBInstanceIdentifier,
						m.HsdpInstanceGuid,
						m.HsdpInstanceName,
						m.SpaceId,
					).Set(floatValue(value))
				}

			}
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	_ = http.ListenAndServe(listenAddr, nil)
}
