<!--
Copyright © 2025-2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Prometheus Metrics

Fabrica automatically generates production-ready Prometheus metrics instrumentation for your REST API when enabled.

## Overview

When metrics are enabled, Fabrica generates:
- **HTTP request metrics** (duration, throughput, status codes)
- **In-flight request tracking** for saturation monitoring
- **Standard Go runtime metrics** (goroutines, memory, GC)
- **Process metrics** (CPU, file descriptors)
- **Build info metrics** for version tracking

All metrics are exposed via a `/metrics` endpoint compatible with Prometheus scraping.

## Quick Start

### Enable Metrics

In your `.fabrica.yaml`:

```yaml
features:
  metrics:
    enabled: true
    provider: prometheus  # Currently only prometheus supported
```

### Regenerate Code

```bash
fabrica generate
```

This creates `cmd/server/metrics_generated.go` with full instrumentation.

### Access Metrics

Start your server and access the metrics endpoint:

```bash
go run ./cmd/server/
curl http://localhost:8080/metrics
```

## Available Metrics

| Metric Name | Type | Labels | Description |
|-------------|------|--------|-------------|
| `{namespace}_http_request_duration_seconds` | Histogram | `method`, `handler`, `code` | Request latency distribution |
| `{namespace}_http_request_total` | Counter | `method`, `handler`, `code` | Total HTTP requests |
| `{namespace}_http_requests_in_flight` | Gauge | - | Current concurrent requests |
| `go_*` | Various | Various | Go runtime metrics (goroutines, memory, GC) |
| `process_*` | Various | Various | Process metrics (CPU, file descriptors) |

**Note**: `{namespace}` is automatically set to your project name (e.g., `my_api`).

## Label Descriptions

- **`method`**: HTTP method (GET, POST, PUT, DELETE, PATCH)
- **`handler`**: Route pattern (e.g., `/users/{id}`, `/devices`)
- **`code`**: HTTP status code (e.g., `200`, `404`, `500`)

## Example Queries

### Request Rate by Endpoint

```promql
rate(my_api_http_request_total[5m])
```

### P99 Latency by Endpoint

```promql
histogram_quantile(0.99,
  rate(my_api_http_request_duration_seconds_bucket[5m])
)
```

### Error Rate (5xx responses)

```promql
sum(rate(my_api_http_request_total{code=~"5.."}[5m]))
/
sum(rate(my_api_http_request_total[5m]))
```

### Current Saturation

```promql
my_api_http_requests_in_flight
```

## Prometheus Configuration

Add this scrape config to your `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'fabrica-api'
    static_configs:
      - targets: ['localhost:8080']
    metrics_path: /metrics
    scrape_interval: 15s
```

## Grafana Dashboard

### Import Pre-Built Dashboard

Fabrica metrics are compatible with standard Go application dashboards:

1. Import dashboard ID `10826` (Go Processes)
2. Import dashboard ID `6671` (Go Metrics)

### Custom Panels

**Request Rate Panel:**
```promql
sum(rate(my_api_http_request_total[5m])) by (handler)
```

**Latency Heatmap:**
```promql
sum(rate(my_api_http_request_duration_seconds_bucket[5m])) by (le, handler)
```

**Error Rate Panel:**
```promql
sum(rate(my_api_http_request_total{code=~"5.."}[5m])) by (handler)
/ ignoring(code) group_left
sum(rate(my_api_http_request_total[5m])) by (handler)
```

## Production Best Practices

### 1. Label Cardinality

**DO NOT** add high-cardinality labels like:
- User IDs
- Request IDs
- Full URLs with parameters
- Timestamps

Labels multiply series, so keep cardinality low.

**Good:**
```
handler="/users/{id}"  # Route pattern
```

**Bad:**
```
handler="/users/12345"  # Unique per user
```

### 2. Histogram Buckets

Default buckets: `[.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10]`

If your SLOs differ, customize in `internal/middleware/metrics_generated.go` after initial generation. Note: Fabrica will regenerate this file, so consider creating a custom metrics file.

### 3. Recording Rules

For frequently queried metrics, use Prometheus recording rules:

```yaml
# prometheus.rules.yml
groups:
  - name: fabrica_api
    interval: 30s
    rules:
      - record: job:http_request_duration_seconds:p99
        expr: histogram_quantile(0.99, sum(rate(my_api_http_request_duration_seconds_bucket[5m])) by (le, job))
```

### 4. Alerting Rules

Example alert for high error rate:

```yaml
# alerts.yml
groups:
  - name: fabrica_api_alerts
    rules:
      - alert: HighErrorRate
        expr: |
          (sum(rate(my_api_http_request_total{code=~"5.."}[5m]))
          /
          sum(rate(my_api_http_request_total[5m]))) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }}"
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
    devicesCreated = promauto.NewCounter(prometheus.CounterOpts{
        Name: "my_api_devices_created_total",
        Help: "Total devices created",
    })

    deviceErrors = promauto.NewCounterVec(prometheus.CounterOpts{
        Name: "my_api_device_errors_total",
        Help: "Total device operation errors",
    }, []string{"operation", "error_type"})
)

func recordDeviceCreation() {
    devicesCreated.Inc()
}

func recordDeviceError(operation, errorType string) {
    deviceErrors.WithLabelValues(operation, errorType).Inc()
}
```

**Important:** Use the `promauto` package for automatic registration. Custom metrics defined this way will automatically appear in `/metrics`.

## Disabling Metrics

To disable metrics:

1. Update `.fabrica.yaml`:
   ```yaml
   features:
     metrics:
       enabled: false
   ```

2. Regenerate:
   ```bash
   fabrica generate
   ```

The metrics middleware and `/metrics` endpoint will be removed from generated code.

## Troubleshooting

### Metrics endpoint returns 404

**Cause**: Metrics disabled in `.fabrica.yaml` or generation failed.

**Fix**:
```bash
# Check config
cat .fabrica.yaml | grep -A2 "metrics:"

# Regenerate
fabrica generate

# Verify file exists
ls internal/middleware/metrics_generated.go
```

### Missing custom metrics

**Cause**: Not using `promauto` or metrics defined after server started.

**Fix**: Use `promauto.NewCounter()` et al. for automatic registration. If you need manual registration, access the registry via the metrics object.

### High cardinality warning

**Cause**: Adding user IDs, request IDs, or other unique values as labels.

**Fix**: Remove high-cardinality labels. Use log correlation instead of metric labels for per-request debugging.

## Architecture

### Metrics Flow

```
HTTP Request
    ↓
Metrics Middleware (start timer, inc in-flight)
    ↓
Application Handler
    ↓
Metrics Middleware (record duration, status, dec in-flight)
    ↓
Response
```

### Registry Design

Fabrica uses a **custom registry** (not the global Prometheus registry) to:
- Avoid test pollution
- Enable metrics reset in tests
- Support multiple generated services in one process

### Generated Files

When metrics are enabled:
- `internal/middleware/metrics_generated.go` - Metrics container and middleware
- `cmd/server/main.go` - Wired into router with `/metrics` endpoint

## Migration from v0.4.x

If you have an existing project with the old metrics stub:

### Before (v0.4.x stub)
```go
// cmd/server/metrics_helpers_generated.go
func metricsHandler(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte("# Metrics would go here\n"))  // Placeholder
}
```

### After (v0.5.0+)
```go
// internal/middleware/metrics_generated.go
type Metrics struct {
    HTTPRequestDuration *prometheus.HistogramVec
    // ... real instrumentation
}
```

### Migration Steps

1. **Backup** (optional, unlikely to have custom changes):
   ```bash
   cp cmd/server/metrics_helpers_generated.go cmd/server/metrics_helpers_backup.go
   ```

2. **Update Fabrica**:
   ```bash
   go install github.com/openchami/fabrica/cmd/fabrica@latest
   ```

3. **Regenerate**:
   ```bash
   fabrica generate
   ```

4. **Verify**:
   ```bash
   go build ./cmd/server
   ./server serve &
   curl http://localhost:8080/metrics
   # Should see real Prometheus metrics
   ```

### Enabling/Disabling Metrics After Initialization

As of Fabrica v0.5.0+, you can toggle metrics on and off by editing `.fabrica.yaml` and running `fabrica generate`. The `cmd/server/main.go` file uses runtime detection to check if metrics are available.

#### Enable Metrics (After Init Without --metrics)

If you initialized your project without the `--metrics` flag and want to enable metrics:

1. **Edit `.fabrica.yaml`**:
   ```yaml
   features:
     metrics:
       enabled: true  # Change from false
       provider: prometheus
   ```

2. **Regenerate code**:
   ```bash
   fabrica generate
   go mod tidy
   ```

3. **Verify**:
   ```bash
   go run ./cmd/server &
   curl http://localhost:8080/metrics
   # Should see Prometheus metrics
   ```

#### Disable Metrics (After Init With --metrics)

If you want to disable metrics:

1. **Edit `.fabrica.yaml`**:
   ```yaml
   features:
     metrics:
       enabled: false  # Change from true
       provider: prometheus
   ```

2. **Regenerate code**:
   ```bash
   fabrica generate
   go mod tidy
   ```

3. **Verify**:
   ```bash
   go run ./cmd/server &
   curl http://localhost:8080/metrics
   # Should return 404 Not Found
   ```

**How It Works**: The generator creates `cmd/server/metrics_helpers_generated.go` with either:
- Real implementation (when enabled) - calls `NewMetrics()` from `metrics_generated.go`
- Stub implementation (when disabled) - returns `nil` and provides no-op methods

The `main.go` file checks for `nil` and conditionally registers the metrics middleware and `/metrics` endpoint.

### Breaking Changes

- **Location**: Metrics moved from `cmd/server/metrics_helpers_generated.go` to `internal/middleware/metrics_generated.go`
- **Endpoint**: Now on main port (`:8080/metrics`) instead of separate port (`:9090/metrics`)
- **Config**: Removed `EnableMetrics` and `MetricsPort` flags (metrics always on main port when enabled)

## Further Reading

- [Prometheus Best Practices](https://prometheus.io/docs/practices/naming/)
- [Prometheus Histograms](https://prometheus.io/docs/practices/histograms/)
- [Go client_golang](https://github.com/prometheus/client_golang)
- [Grafana Dashboards](https://grafana.com/grafana/dashboards/)

## See Also

- [Validation](./validation.md) - Request validation
- [Events](./events.md) - CloudEvents integration
- [Middleware](./middleware.md) - Custom middleware
