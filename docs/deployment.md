# Deployment Guide

Production deployment guide for IncidentGarden.

## Environment Variables

### Required

| Variable | Description | Example |
|----------|-------------|---------|
| `DATABASE_URL` | PostgreSQL connection string | `postgres://user:pass@host:5432/db?sslmode=require` |
| `JWT_SECRET_KEY` | JWT signing key (min 32 chars) | `<random-32-char-string>` |

### Optional (with defaults)

| Variable | Default | Description |
|----------|---------|-------------|
| `SERVER_HOST` | `0.0.0.0` | Bind address |
| `SERVER_PORT` | `8080` | HTTP port |
| `SERVER_METRICS_PORT` | `9090` | Port for Prometheus metrics endpoint |
| `SERVER_READ_TIMEOUT` | `15s` | HTTP read timeout |
| `SERVER_READ_HEADER_TIMEOUT` | `5s` | Max time to read request headers (Slowloris protection) |
| `SERVER_WRITE_TIMEOUT` | `15s` | HTTP write timeout |
| `SERVER_IDLE_TIMEOUT` | `60s` | Max time to keep idle connections open |
| `DATABASE_MAX_OPEN_CONNS` | `25` | Max DB connections |
| `DATABASE_MAX_IDLE_CONNS` | `5` | Min idle connections |
| `DATABASE_CONN_MAX_LIFETIME` | `5m` | Connection reuse time |
| `DATABASE_CONNECT_TIMEOUT` | `30s` | Total timeout for initial DB connection (including retries) |
| `DATABASE_CONNECT_ATTEMPTS` | `5` | Number of connection attempts with exponential backoff |
| `LOG_LEVEL` | `info` | debug, info, warn, error |
| `LOG_FORMAT` | `json` | json or text |
| `CORS_ALLOWED_ORIGINS` | `http://localhost:3000` | Comma-separated origins |
| `COOKIE_SECURE` | `false` | Set true in production (HTTPS) |
| `COOKIE_DOMAIN` | `` | Cookie domain |

## Health Endpoints

| Endpoint | Purpose | Use as |
|----------|---------|--------|
| `GET /healthz` | Process alive check | Liveness probe |
| `GET /readyz` | DB connectivity check (2s timeout) | Readiness probe, Startup probe |
| `GET /version` | Build info (version, commit, date) | Informational |

## Kubernetes Configuration

### Probes

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 10
  periodSeconds: 30
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 10
  failureThreshold: 3

# Optional: use if app takes long to start (DB migrations, etc.)
startupProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5
  failureThreshold: 30  # 150s max startup time
```

### Resources

Recommended starting point (adjust based on actual usage):

```yaml
resources:
  requests:
    memory: "128Mi"
    cpu: "100m"
  limits:
    memory: "512Mi"
    cpu: "500m"
```

### Graceful Shutdown

Application handles `SIGTERM` with 10-second timeout. For proper load balancer draining:

```yaml
spec:
  terminationGracePeriodSeconds: 30
  containers:
  - name: statuspage
    lifecycle:
      preStop:
        exec:
          command: ["sleep", "5"]  # Wait for LB to remove pod from rotation
```

### Secrets

Store sensitive values in Kubernetes Secrets:

```yaml
env:
- name: JWT_SECRET_KEY
  valueFrom:
    secretKeyRef:
      name: incident-garden-secrets
      key: jwt-secret
- name: DATABASE_URL
  valueFrom:
    secretKeyRef:
      name: incident-garden-secrets
      key: database-url
```

### Example Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: incident-garden
spec:
  replicas: 2
  selector:
    matchLabels:
      app: incident-garden
  template:
    metadata:
      labels:
        app: incident-garden
    spec:
      terminationGracePeriodSeconds: 30
      containers:
      - name: statuspage
        image: ghcr.io/bissquit/incident-garden:latest
        ports:
        - containerPort: 8080
          name: http
        - containerPort: 9090
          name: metrics
        env:
        - name: LOG_LEVEL
          value: "info"
        - name: LOG_FORMAT
          value: "json"
        - name: JWT_SECRET_KEY
          valueFrom:
            secretKeyRef:
              name: incident-garden-secrets
              key: jwt-secret
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: incident-garden-secrets
              key: database-url
        livenessProbe:
          httpGet:
            path: /healthz
            port: 8080
          initialDelaySeconds: 10
          periodSeconds: 30
          failureThreshold: 3
        readinessProbe:
          httpGet:
            path: /readyz
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 10
          failureThreshold: 3
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        lifecycle:
          preStop:
            exec:
              command: ["sleep", "5"]
```

## Database

- **PostgreSQL 15+** required
- Migrations located at `/app/migrations` inside container
- Run migrations before starting application (init container or external job)
- Use `?sslmode=require` in production DATABASE_URL

### Migration Job Example

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: incident-garden-migrate
spec:
  template:
    spec:
      containers:
      - name: migrate
        image: migrate/migrate:v4.16.2
        command:
        - migrate
        - -path=/migrations
        - -database=$(DATABASE_URL)
        - up
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: incident-garden-secrets
              key: database-url
        volumeMounts:
        - name: migrations
          mountPath: /migrations
      volumes:
      - name: migrations
        configMap:
          name: incident-garden-migrations
      restartPolicy: OnFailure
```

## Ports

| Port | Protocol | Description |
|------|----------|-------------|
| 8080 | HTTP | API, health checks (/healthz, /readyz) |
| 9090 | HTTP | Prometheus metrics (/metrics) |

## Docker Image

```
ghcr.io/bissquit/incident-garden:latest
```

Image details:
- Base: `alpine:3.19`
- Runs as non-root user (UID 1000)
- Contains: binary, migrations, OpenAPI spec
- Built with `CGO_ENABLED=0` (static binary)

## Logging

Application outputs JSON-formatted logs to stdout (default). Example:

```json
{"time":"2024-01-15T10:30:45Z","level":"INFO","msg":"starting server","host":"0.0.0.0","port":"8080"}
```

Configure log aggregation (Loki, ELK, CloudWatch) to collect from stdout.

## Monitoring

### Metrics Endpoint

Application exposes Prometheus metrics on a **separate port**:

| Port | Endpoint | Description |
|------|----------|-------------|
| 8080 | `/healthz`, `/readyz`, `/api/v1/*` | API and health checks |
| 9090 | `/metrics` | Prometheus metrics |

### Available Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `incidentgarden_http_request_duration_seconds` | Histogram | method, route, status_code | HTTP request latency |
| `incidentgarden_db_pool_connections` | Gauge | state (in_use, idle, max) | Database connection pool |
| `go_*`, `process_*` | Various | — | Go runtime metrics |

### Kubernetes Service

```yaml
ports:
  - name: http
    port: 8080
    targetPort: 8080
  - name: metrics
    port: 9090
    targetPort: 9090
```

### ServiceMonitor

Pre-configured ServiceMonitor is available in `deployments/prometheus/servicemonitor.yaml`.

```bash
# Apply after adjusting namespace and labels
kubectl apply -f deployments/prometheus/servicemonitor.yaml
```

### NetworkPolicy

Example NetworkPolicy for isolating metrics port is in `deployments/prometheus/networkpolicy-example.yaml`.

### Prometheus Scrape Config

```yaml
scrape_configs:
  - job_name: 'incidentgarden'
    static_configs:
      - targets: ['<host>:9090']
    metrics_path: /metrics
    scrape_interval: 15s
```

### Alerts

Pre-configured alerts are available in `deployments/prometheus/alerts.yaml`.

| Alert | Condition | Severity |
|-------|-----------|----------|
| HighErrorRate | Error rate > 5% for 5m | critical |
| HighLatency | p99 > 2s for 5m | warning |
| DBPoolExhaustion | Pool > 80% for 5m | warning |
| NoRequests | No traffic for 10m | warning |
| HighMemory | Memory > 400MB for 10m | warning |
| GoroutineLeak | Goroutines > 1000 for 10m | warning |
| ReadinessFailing | Scrape fails for 2m | critical |

### Health Endpoints

- `/healthz` - synthetic monitoring, uptime checks
- `/readyz` - dependency health (returns 503 if DB unavailable)
- `/version` - deployment verification

### Recommended Additional Alerts

| Condition | Severity | Description |
|-----------|----------|-------------|
| `/readyz` returns 503 | Critical | Database connectivity lost |
| Pod restart count > 3/hour | Warning | Application instability |
| Memory usage > 80% limit | Warning | Potential OOM |
