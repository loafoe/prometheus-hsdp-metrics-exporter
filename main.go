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
	flag.StringVar(&region, "region", "us-east", "The HSDP region to use")
	flag.StringVar(&listenAddr, "listen", "0.0.0.0:8889", "Listen address for HTTP metrics")
	flag.Parse()

	uaaUsername := os.Getenv("UAA_USERNAME")
	uaaPassword := os.Getenv("UAA_PASSWORD")

	if debugLog == "" {
		debugLog = os.Getenv("DEBUG_LOG")
	}

	if envRegion := os.Getenv("HSDP_REGION"); region != "" {
		region = envRegion
	}

	if uaaUsername == "" || uaaPassword == "" {
		fmt.Printf("missing UAA_USERNAME or UAA_PASSWORD environment values\n")
		os.Exit(1)
	}

	fmt.Printf("connecting to HSDP region %s\n", region)
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

	// RDS
	rdsCPMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_cpu_average"),
		hsdp.WithHelp("HSDP RDS database CPU utilization average"),
		hsdp.WithQuery("(aws_rds_cpuutilization_average) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsDatabaseConnectionsMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_database_connections_average"),
		hsdp.WithHelp("The average number of database connections"),
		hsdp.WithQuery("(aws_rds_database_connections_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeStorageMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_free_storage_space_average"),
		hsdp.WithHelp("The average free storage space"),
		hsdp.WithQuery("(aws_rds_free_storage_space_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeableMemoryMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_freeable_memory_average"),
		hsdp.WithHelp("The average freeable memory"),
		hsdp.WithQuery("(aws_rds_freeable_memory_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsReadIOPSMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_read_iops_average"),
		hsdp.WithHelp("The average read operations"),
		hsdp.WithQuery("(aws_rds_read_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsWriteOPSMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithName("rds_write_iops_average"),
		hsdp.WithHelp("Average write operations"),
		hsdp.WithQuery("(aws_rds_write_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	// S3
	s3BucketSizeMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("s3"),
		hsdp.WithRegion(region),
		hsdp.WithName("s3_bucket_size"),
		hsdp.WithHelp("The total bucket size"),
		hsdp.WithQuery("(s3_bucket_size)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	s3ObjectsMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("s3"),
		hsdp.WithRegion(region),
		hsdp.WithName("s3_objects"),
		hsdp.WithHelp("The total number of objects in the bucket"),
		hsdp.WithQuery("(s3_objects)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	registry.MustRegister(rdsCPMetric)
	registry.MustRegister(rdsDatabaseConnectionsMetric)
	registry.MustRegister(rdsFreeStorageMetric)
	registry.MustRegister(rdsFreeableMemoryMetric)
	registry.MustRegister(rdsReadIOPSMetric)
	registry.MustRegister(rdsWriteOPSMetrics)
	registry.MustRegister(s3BucketSizeMetrics)
	registry.MustRegister(s3ObjectsMetrics)

	go func() {
		sleep := false
		for {
			if sleep {
				time.Sleep(time.Second * 30)
			}
			sleep = true
			instances, err := uaaClient.Metrics.GQLGetInstances(ctx)
			if err != nil {
				fmt.Printf("error fetching instances: %v\n", err)
				continue
			}
			if len(*instances) == 0 {
				fmt.Printf("no metrics instances found\n")
			}
			for _, instance := range *instances {
				fmt.Printf("Instance: %+v\n", instance.GUID)
				_ = rdsCPMetric.Update(ctx, instance)
				_ = rdsDatabaseConnectionsMetric.Update(ctx, instance)
				_ = rdsFreeStorageMetric.Update(ctx, instance)
				_ = rdsFreeableMemoryMetric.Update(ctx, instance)
				_ = rdsReadIOPSMetric.Update(ctx, instance)
				_ = rdsWriteOPSMetrics.Update(ctx, instance)
				_ = s3BucketSizeMetrics.Update(ctx, instance)
				_ = s3ObjectsMetrics.Update(ctx, instance)
			}
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	_ = http.ListenAndServe(listenAddr, nil)
}
