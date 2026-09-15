# Prometheus metrics

The server exposes Prometheus metrics at `GET /metrics`. The scrape request is
not included in HTTP request metrics, so monitoring does not generate its own
traffic. HTTP labels use Chi route templates, normalized methods, and response
status codes; repository labels use generated Go method names and the fixed
`success`/`error` result. Raw paths, SQL, request values, user IDs, and slugs are
never labels, so metric cardinality remains bounded.

## HTTP requests

Requests per second by method and route:

```promql
sum by (method, route) (rate(conduit_http_requests_total[1m]))
```

Response codes by method and route:

```promql
sum by (method, route, status) (rate(conduit_http_requests_total[5m]))
```

Server errors per second:

```promql
sum by (method, route) (rate(conduit_http_requests_total{status=~"5.."}[5m]))
```

95th-percentile response time:

```promql
histogram_quantile(
  0.95,
  sum by (le, method, route) (rate(conduit_http_request_duration_seconds_bucket[5m]))
)
```

95th-percentile response size:

```promql
histogram_quantile(
  0.95,
  sum by (le, method, route) (rate(conduit_http_response_size_bytes_bucket[5m]))
)
```

Currently running requests:

```promql
conduit_http_requests_in_flight
```

Recovered handler panics during the last five minutes:

```promql
sum by (method, route) (increase(conduit_http_panics_total[5m]))
```

## Repository queries

Requests per second by repository method:

```promql
sum by (method) (rate(conduit_repository_method_calls_total[1m]))
```

Errors per method during the last five minutes:

```promql
sum by (method) (increase(conduit_repository_method_calls_total{result="error"}[5m]))
```

95th-percentile execution time by method:

```promql
histogram_quantile(
  0.95,
  sum by (le, method) (rate(conduit_repository_method_duration_seconds_bucket[5m]))
)
```

Average execution time by method:

```promql
sum by (method) (rate(conduit_repository_method_duration_seconds_sum[5m]))
/
sum by (method) (rate(conduit_repository_method_duration_seconds_count[5m]))
```
