package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/loafoe/prometheus-hsdp-metrics-exporter/hsdp"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/philips-software/go-hsdp-api/console"
)

var region string
var listenInterface string
var listenPort int
var debug bool
var refresh int
var prune int

func main() {
	flag.BoolVar(&debug, "debug", false, "Enable debugging")
	flag.StringVar(&region, "region", "us-east", "The HSDP region to use")
	flag.IntVar(&listenPort, "port", 8889, "Listen on this port")
	flag.StringVar(&listenInterface, "iface", "0.0.0.0", "Listen interface for HTTP metrics")
	flag.IntVar(&refresh, "refresh", 60, "The time to wait between refreshes")
	flag.IntVar(&prune, "prune", 120, "The time to wait before pruning stale instances")

	flag.Parse()

	uaaUsername := os.Getenv("UAA_USERNAME")
	uaaPassword := os.Getenv("UAA_PASSWORD")

	if prune <= refresh {
		fmt.Printf("prune cannot be less than refresh (%d <= %d), we will have nothing to show\n", prune, refresh)
		os.Exit(1)
	}

	if envRegion := os.Getenv("HSDP_REGION"); envRegion != "" {
		region = envRegion
	}
	if port := os.Getenv("PORT"); port != "" {
		parsedPort, err := strconv.ParseInt(port, 10, 64)
		if err != nil {
			fmt.Printf("Invalid port: %s\n", port)
			os.Exit(1)
		}
		listenPort = int(parsedPort)
	}
	if iface := os.Getenv("INTERFACE"); iface != "" {
		listenInterface = iface
	}

	if uaaUsername == "" || uaaPassword == "" {
		fmt.Printf("missing UAA_USERNAME or UAA_PASSWORD environment values\n")
		os.Exit(1)
	}

	fmt.Printf("Connecting to HSDP region %s as %s\n", region, uaaUsername)
	clientConfig := console.Config{
		Region: region,
	}
	if debug {
		clientConfig.DebugLog = os.Stderr
	}
	uaaClient, err := console.NewClient(nil, &clientConfig)
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
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_cpu_average"),
		hsdp.WithHelp("HSDP RDS database CPU utilization average"),
		hsdp.WithQuery("(aws_rds_cpuutilization_average) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsDatabaseConnectionsMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_database_connections_average"),
		hsdp.WithHelp("The average number of database connections"),
		hsdp.WithQuery("(aws_rds_database_connections_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeStorageMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_free_storage_space_average"),
		hsdp.WithHelp("The average free storage space"),
		hsdp.WithQuery("(aws_rds_free_storage_space_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsFreeableMemoryMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_freeable_memory_average"),
		hsdp.WithHelp("The average freeable memory"),
		hsdp.WithQuery("(aws_rds_freeable_memory_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsReadIOPSMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_read_iops_average"),
		hsdp.WithHelp("The average read operations"),
		hsdp.WithQuery("(aws_rds_read_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))
	rdsWriteOPSMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rds"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rds_write_iops_average"),
		hsdp.WithHelp("Average write operations"),
		hsdp.WithQuery("(aws_rds_write_iops_average)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	// S3
	s3BucketSizeMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("s3"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("s3_bucket_size"),
		hsdp.WithHelp("The total bucket size"),
		hsdp.WithQuery("(s3_bucket_size)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	s3ObjectsMetrics, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("s3"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("s3_objects"),
		hsdp.WithHelp("The total number of objects in the bucket"),
		hsdp.WithQuery("(s3_objects)  * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	// RabbitMQ
	rabbitmqQueuedMessagesTotalMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rabbitmq"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rabbitmq_queued_messages_total"),
		hsdp.WithHelp("The total number of queued messages"),
		hsdp.WithQuery("(rabbitmq_queue_messages_total) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"),
	)
	rabbitmqExchangesTotalMetric, _ := hsdp.NewMetric(
		hsdp.WithClient(uaaClient),
		hsdp.WithService("rabbitmq"),
		hsdp.WithRegion(region),
		hsdp.WithPrune(prune),
		hsdp.WithName("rabbitmq_exchanges_total"),
		hsdp.WithHelp("Total number of exchanges"),
		hsdp.WithQuery("(rabbitmq_exchangesTotal) * on (hsdp_instance_guid) group_left(hsdp_instance_name)(cf_service_instance_info{hsdp_instance_name=~\".*\"})"))

	registry.MustRegister(rdsCPMetric)
	registry.MustRegister(rdsDatabaseConnectionsMetric)
	registry.MustRegister(rdsFreeStorageMetric)
	registry.MustRegister(rdsFreeableMemoryMetric)
	registry.MustRegister(rdsReadIOPSMetric)
	registry.MustRegister(rdsWriteOPSMetrics)
	registry.MustRegister(s3BucketSizeMetrics)
	registry.MustRegister(s3ObjectsMetrics)
	registry.MustRegister(rabbitmqQueuedMessagesTotalMetric)
	registry.MustRegister(rabbitmqExchangesTotalMetric)

	go func() {
		sleep := false
		for {
			if sleep {
				time.Sleep(time.Second * time.Duration(refresh))
			}
			sleep = true
			instances, err := uaaClient.Metrics.GQLGetInstances(ctx)
			if err != nil {
				fmt.Printf("error fetching instances: %v\n", err)
				continue
			}
			if found := len(*instances); found == 0 {
				fmt.Printf("no metrics instances found\n")
			} else {
				fmt.Printf("%3d metrics instances found\n", found)
			}
			for _, instance := range *instances {
				//fmt.Printf("Instance: %+v\n", instance.GUID)
				_ = rdsCPMetric.Update(ctx, instance)
				_ = rdsDatabaseConnectionsMetric.Update(ctx, instance)
				_ = rdsFreeStorageMetric.Update(ctx, instance)
				_ = rdsFreeableMemoryMetric.Update(ctx, instance)
				_ = rdsReadIOPSMetric.Update(ctx, instance)
				_ = rdsWriteOPSMetrics.Update(ctx, instance)
				_ = s3BucketSizeMetrics.Update(ctx, instance)
				_ = s3ObjectsMetrics.Update(ctx, instance)
				_ = rabbitmqQueuedMessagesTotalMetric.Update(ctx, instance)
				_ = rabbitmqExchangesTotalMetric.Update(ctx, instance)
			}
		}
	}()

	http.Handle("/metrics", promhttp.HandlerFor(registry, promhttp.HandlerOpts{}))

	_ = http.ListenAndServe(fmt.Sprintf("%s:%d", listenInterface, listenPort), nil)
}
