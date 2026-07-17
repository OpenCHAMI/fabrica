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

**Note:** Make sure to add `import "time"` at the top of the file since `SensorStatus` uses `time.Time`.

```go
type SensorSpec struct {
    Location    string  `json:"location" validate:"required"`
    Temperature float64 `json:"temperature" validate:"required"`
    Humidity    float64 `json:"humidity" validate:"required,min=0,max=100"`
    Status      string  `json:"status" validate:"required,oneof=active inactive"`
}

type SensorStatus struct {
    LastReading time.Time `json:"lastReading"` // Requires: import "time"
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

You should see Prometheus-format metrics including HTTP request counters, latency histograms, in-flight requests, and Go runtime metrics.

## Metrics Reference

For complete metrics documentation, see the **[Prometheus Metrics Guide](../../docs/guides/metrics.md)**, which includes:

- **Complete metrics table** - All available metrics with descriptions
- **PromQL query examples** - Request rate, P99 latency, error rate, and more
- **Prometheus/Grafana configuration** - Production monitoring setup
- **Best practices** - Label cardinality, histogram buckets, aggregation
- **Troubleshooting** - Common issues and solutions
- **Migration guide** - Upgrading from previous versions

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

## Grafana Dashboard

See the [Metrics Guide](../../docs/guides/metrics.md#grafana-setup) for:
- Dashboard configuration
- Pre-built panel examples
- Prometheus data source setup

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

See the [Metrics Guide](../../docs/guides/metrics.md#custom-metrics) for how to add custom business metrics to your handlers.

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

For common issues and solutions, see the [Metrics Guide Troubleshooting section](../../docs/guides/metrics.md#troubleshooting).

## See Also

- [Metrics Guide](../../docs/guides/metrics.md) - Complete documentation
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Dashboards](https://grafana.com/grafana/dashboards/)
- [Example 05: CloudEvents](../05-cloud-events/) - Event-driven architecture
- [Example 03: FRU Service](../03-fru-service/) - Production patterns
