# prometheus-hsdp-metrics-exporter

Prometheus exporter for HSP Metrics. It allows one to consolidate metrics collection across the HSP landscape by
presenting the HSP Metrics console data as (re-)scrapable endpoint. If you need additional flexibility not
offered by the HSP Console UI consider using this exporter.

## Install

Using Go 1.18 or newer

```shell
go install github.com/loafoe/prometheus-hsdp-metrics-exporter@latest
```

## Usage

### Set credentials and region

```shell
export UAA_USERNAME=your-uaa-username
export UAA_PASSWORD=your-uaa-password
export HSDP_REGION=us-east
```

### Run exporter

```shell
promethues-hsdp-metrics-exporter -listen 0.0.0.0:8889
```

### Ship to prometheus

You can use something like Grafana-agent to ship data to a remote write endpoint. Example:

```yml
metrics:
  global:
    scrape_interval: 1m
    external_labels:
      environment: p1-server
  configs:
    - name: default
      scrape_configs:
        - job_name: 'hsdp_metrics_exporter'
          static_configs:
            - targets: ['localhost:8889']
      remote_write:
        - url: https://prometheus.example.com/api/v1/write
          basic_auth:
            username: scraper
            password: S0m3pAssW0rdH3Re
```

## License

License is MIT
