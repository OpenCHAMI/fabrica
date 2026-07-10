<!--
Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Example 06: Prometheus Metrics

This example demonstrates Fabrica's Prometheus metrics instrumentation for production monitoring.

## What You'll Learn

- Enable and configure Prometheus metrics
- Access the `/metrics` endpoint
- Understand generated metrics (HTTP, storage, events)
- Query metrics with PromQL
- Set up Prometheus scraping
- Create Grafana dashboards

## Prerequisites

- Fabrica CLI installed
- Go 1.26.4+ installed
- (Optional) Prometheus and Grafana for visualization

## Quick Start

### 1. Create the Project

```bash
# Create project with metrics enabled
fabrica init sensor-api --module github.com/example/sensor-api --events --events-bus memory

cd sensor-api
```

### 2. Enable Metrics

Edit `.fabrica.yaml`:

```yaml
features:
  metrics:
    enabled: true
    provider: prometheus
```

### 3. Add a Resource

```bash
fabrica add resource Sensor
```

Edit `apis/example.fabrica.dev/v1/sensor_types.go`:

```go
type SensorSpec struct {
    Location    string  `json:"location" validate:"required"`
    Temperature float64 `json:"temperature" validate:"required"`
    Humidity    float64 `json:"humidity" validate:"required,min=0,max=100"`
    Status      string  `json:"status" validate:"required,oneof=active inactive"`
}

type SensorStatus struct {
    LastReading time.Time `json:"lastReading"`
    Health      string    `json:"health" validate:"oneof=healthy degraded failed"`
    ErrorCount  int       `json:"errorCount"`
}
```

### 4. Generate Code

```bash
fabrica generate
go mod tidy
```

### 5. Run the Server

```bash
go run ./cmd/server/
```

### 6. Access Metrics

```bash
curl http://localhost:8080/metrics
```

You should see output like:

```
# HELP sensor_api_http_request_duration_seconds HTTP request latency in seconds.
# TYPE sensor_api_http_request_duration_seconds histogram
sensor_api_http_request_duration_seconds_bucket{code="200",handler="/sensors",method="GET",le="0.005"} 1
sensor_api_http_request_duration_seconds_bucket{code="200",handler="/sensors",method="GET",le="0.01"} 1
...

# HELP sensor_api_http_request_total Total HTTP requests.
# TYPE sensor_api_http_request_total counter
sensor_api_http_request_total{code="200",handler="/sensors",method="GET"} 1

# HELP sensor_api_http_requests_in_flight Current in-flight HTTP requests.
# TYPE sensor_api_http_requests_in_flight gauge
sensor_api_http_requests_in_flight 0

# HELP sensor_api_storage_operation_duration_seconds Storage operation latency in seconds.
# TYPE sensor_api_storage_operation_duration_seconds histogram
sensor_api_storage_operation_duration_seconds_bucket{operation="save",resource_type="Sensor",le="0.001"} 1
...

# HELP sensor_api_events_published_total Total events published to the event bus.
# TYPE sensor_api_events_published_total counter
sensor_api_events_published_total{event_type="com.fabrica.Sensor.created",status="success"} 1
```

## Testing the API

### Create a Sensor

```bash
curl -X POST http://localhost:8080/sensors \
  -H "Content-Type: application/json" \
  -d '{
    "apiVersion": "example.fabrica.dev/v1",
    "kind": "Sensor",
    "metadata": {
      "name": "sensor-01"
    },
    "spec": {
      "location": "Lab A",
      "temperature": 22.5,
      "humidity": 45,
      "status": "active"
    },
    "status": {
      "lastReading": "2026-07-10T12:00:00Z",
      "health": "healthy",
      "errorCount": 0
    }
  }'
```

### List Sensors

```bash
curl http://localhost:8080/sensors
```

### Check Metrics Again

```bash
curl http://localhost:8080/metrics | grep sensor_api_http_request_total
```

You should see counters incremented for your requests.

## Available Metrics

### HTTP Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sensor_api_http_request_duration_seconds` | Histogram | method, handler, code | Request latency |
| `sensor_api_http_request_total` | Counter | method, handler, code | Total requests |
| `sensor_api_http_requests_in_flight` | Gauge | - | Current requests |

### Storage Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sensor_api_storage_operation_duration_seconds` | Histogram | operation, resource_type | Storage latency |
| `sensor_api_storage_operation_total` | Counter | operation, resource_type, status | Total operations |

Operations: `load`, `load_all`, `save`, `update`, `delete`

### Event Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `sensor_api_events_published_total` | Counter | event_type, status | Events published |
| `sensor_api_event_subscribers` | Gauge | - | Active subscribers |

### Go Runtime Metrics

Standard metrics automatically included:
- `go_goroutines` - Current goroutines
- `go_memstats_alloc_bytes` - Allocated memory
- `go_gc_duration_seconds` - GC pause duration
- `process_cpu_seconds_total` - CPU time
- `process_resident_memory_bytes` - RSS memory

## Prometheus Configuration

Create `prometheus.yml`:

```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'sensor-api'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
```

Run Prometheus:

```bash
prometheus --config.file=prometheus.yml
```

Access Prometheus UI: http://localhost:9090

## Example Queries

### Request Rate (QPS)

```promql
rate(sensor_api_http_request_total[5m])
```

### P99 Latency

```promql
histogram_quantile(0.99,
  rate(sensor_api_http_request_duration_seconds_bucket[5m])
)
```

### Error Rate

```promql
sum(rate(sensor_api_http_request_total{code=~"5.."}[5m]))
/
sum(rate(sensor_api_http_request_total[5m]))
```

### Storage Operation Breakdown

```promql
rate(sensor_api_storage_operation_total[5m])
```

### Event Publishing Rate

```promql
rate(sensor_api_events_published_total{status="success"}[5m])
```

### Current Load

```promql
sensor_api_http_requests_in_flight
```

## Grafana Dashboard

Import the pre-built dashboard:

1. Install Grafana
2. Add Prometheus as data source (http://localhost:9090)
3. Import dashboard JSON (see `dashboard.json` in this directory)

Or create panels manually with the queries above.

## Load Testing

Generate traffic to see metrics in action:

```bash
# Install hey (HTTP load generator)
go install github.com/rakyll/hey@latest

# Generate load
hey -z 60s -c 10 http://localhost:8080/sensors
```

Watch metrics update in real-time:

```bash
watch -n 1 'curl -s http://localhost:8080/metrics | grep -E "(request_total|requests_in_flight)"'
```

## Custom Metrics

To add custom business metrics, create `cmd/server/custom_metrics.go`:

```go
package main

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    sensorReadingsTotal = promauto.NewCounter(prometheus.CounterOpts{
        Name: "sensor_api_readings_total",
        Help: "Total sensor readings processed",
    })

    sensorTemperatureGauge = promauto.NewGaugeVec(prometheus.GaugeOpts{
        Name: "sensor_api_current_temperature",
        Help: "Current temperature by sensor",
    }, []string{"sensor_name"})
)

// Call in your handlers
func recordReading(sensorName string, temp float64) {
    sensorReadingsTotal.Inc()
    sensorTemperatureGauge.WithLabelValues(sensorName).Set(temp)
}
```

## Best Practices

### DO

- Keep label cardinality low (< 1000 unique combinations)
- Use histograms for latency (not summaries)
- Set histogram buckets based on SLOs
- Export metrics on the main port (easier for Kubernetes)
- Monitor both success and error rates

### DON'T

- Add user IDs or request IDs as labels (too high cardinality)
- Use summaries for request latency (can't aggregate)
- Expose sensitive data in metric labels
- Create unbounded label values

## Disabling Metrics

To disable metrics:

1. Edit `.fabrica.yaml`:
   ```yaml
   features:
     metrics:
       enabled: false
   ```

2. Regenerate:
   ```bash
   fabrica generate
   ```

The `/metrics` endpoint and instrumentation will be removed.

## Troubleshooting

### Metrics endpoint returns 404

Metrics are disabled. Check `.fabrica.yaml` and regenerate.

### Missing custom metrics

Use `promauto` for automatic registration. Custom metrics defined with `promauto.NewCounter()` etc. will appear automatically.

### High memory usage

Check for label cardinality explosion:

```promql
# Count unique series
count({__name__=~"sensor_api_.*"})

# By metric
count by (__name__) ({__name__=~"sensor_api_.*"})
```

## See Also

- [Metrics Guide](../../docs/guides/metrics.md) - Complete documentation
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Dashboards](https://grafana.com/grafana/dashboards/)
- [Example 05: CloudEvents](../05-cloud-events/) - Event-driven architecture
- [Example 03: FRU Service](../03-fru-service/) - Production patterns
