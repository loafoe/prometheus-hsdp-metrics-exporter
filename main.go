package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/loafoe/prometheus-hsdp-metrics-exporter/hsdp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/philips-software/go-hsdp-api/console"
)

var region string
var listenAddr string
var debugLog string

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

	registry := prometheus.NewRegistry()

	rdsCPMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_cpu_average"),
		hsdp.WithHelp("HSDP RDS database CPU utilization average"),
		hsdp.WithQuery("(aws_rds_cpuutilization_average) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsDatabaseConnectionsMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_database_connections_average"),
		hsdp.WithHelp("The average number of database connections"),
		hsdp.WithQuery("(aws_rds_database_connections_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeStorageMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_free_storage_space_average"),
		hsdp.WithHelp("The average free storage space"),
		hsdp.WithQuery("(aws_rds_free_storage_space_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeableMemoryMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_freeable_memory_average"),
		hsdp.WithHelp("The average freeable memory"),
		hsdp.WithQuery("(aws_rds_freeable_memory_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsReadIOPSMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_read_iops_average"),
		hsdp.WithHelp("The average read operations"),
		hsdp.WithQuery("(aws_rds_read_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsWriteOPSMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithName("rds_write_iops_average"),
		hsdp.WithHelp("Average write operations"),
		hsdp.WithQuery("(aws_rds_write_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	registry.MustRegister(rdsCPMetric)
	registry.MustRegister(rdsDatabaseConnectionsMetric)
	registry.MustRegister(rdsFreeStorageMetric)
	registry.MustRegister(rdsFreeableMemoryMetric)
	registry.MustRegister(rdsReadIOPSMetric)
	registry.MustRegister(rdsWriteOPSMetrics)

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
				_ = rdsCPMetric.Update(ctx, instance)
				_ = rdsDatabaseConnectionsMetric.Update(ctx, instance)
				_ = rdsFreeStorageMetric.Update(ctx, instance)
				_ = rdsFreeableMemoryMetric.Update(ctx, instance)
				_ = rdsReadIOPSMetric.Update(ctx, instance)
				_ = rdsWriteOPSMetrics.Update(ctx, instance)
			}
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	_ = http.ListenAndServe(listenAddr, nil)
}
