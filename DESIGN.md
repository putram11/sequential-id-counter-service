# Sequential ID Counter Service - Complete Design

## Executive Summary

Enterprise-grade sequential ID counter service providing atomic, scalable, and auditable ID generation (e.g., SG000001, INV2025-0001). Uses Redis for fast counters, RabbitMQ for reliable event buffering, and PostgreSQL for audit trails and configuration management.

## Architecture Overview

```
[Clients/ERP Apps]
        |
        v
[gRPC/REST API Gateway]
        |
   +----+--------+
   |             |
[API Service 1] [API Service N] (stateless Go services)
   |             |
   +-------> Redis Cluster (counters)
   |
   +-------> RabbitMQ Cluster (event logs)
                 |
           +-----+-----+
           |           |
      [Worker 1]   [Worker M] -> PostgreSQL (audit/config)
                                      |
                                [Monitoring Stack]
```

## 1. Component Architecture

### 1.1 API Service (Stateless)
- **Technology**: Go with Gin (REST) + gRPC
- **Responsibilities**:
  - Increment Redis counters atomically
  - Format IDs according to prefix configuration
  - Publish events to RabbitMQ
  - Provide fast responses to clients
- **Endpoints**:
  - gRPC: `GetNext`, `ResetCounter`, `GetStatus`, `UpdateConfig`
  - REST: `GET /api/v1/next/:prefix`, `POST /api/v1/reset/:prefix`

### 1.2 Redis (Counter Store)
- **Configuration**: Cluster mode with replicas
- **Data Structure**: `seq:<prefix>` → counter value
- **Persistence**: AOF + RDB snapshots
- **HA**: Primary-replica with automatic failover

### 1.3 RabbitMQ (Event Queue)
- **Setup**: Clustered with mirrored queues
- **Exchange**: `seq_exchange` (direct)
- **Queue**: `seq_log_queue` (durable)
- **Messages**: Persistent JSON payloads

### 1.4 Workers (Event Consumers)
- **Technology**: Go consumers
- **Responsibilities**: 
  - Consume events from RabbitMQ
  - Insert audit records to PostgreSQL
  - Handle retries and dead letter queues

### 1.5 PostgreSQL (Source of Truth)
- **Tables**: `seq_config`, `seq_log`, `seq_checkpoint`
- **Features**: ACID transactions, audit trails, configuration management
- **HA**: Primary-replica with streaming replication

## 2. Data Models

### 2.1 Database Schema

```sql
-- Configuration table
CREATE TABLE seq_config (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) UNIQUE NOT NULL,
    padding_length INTEGER NOT NULL DEFAULT 6,
    format_template TEXT NOT NULL DEFAULT '%s%0*d',
    reset_rule VARCHAR(20) NOT NULL DEFAULT 'never',
    last_reset_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Audit log table
CREATE TABLE seq_log (
    id BIGSERIAL PRIMARY KEY,
    prefix VARCHAR(50) NOT NULL,
    counter_value BIGINT NOT NULL,
    full_number VARCHAR(255) NOT NULL,
    generated_by VARCHAR(100),
    client_id VARCHAR(100),
    generated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    published_at TIMESTAMP WITH TIME ZONE,
    inserted_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    UNIQUE(prefix, counter_value)
);

-- Checkpoint table for fast recovery
CREATE TABLE seq_checkpoint (
    prefix VARCHAR(50) PRIMARY KEY,
    last_counter_synced BIGINT NOT NULL,
    synced_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Indexes
CREATE INDEX idx_seq_log_prefix_counter ON seq_log(prefix, counter_value);
CREATE INDEX idx_seq_log_generated_at ON seq_log(generated_at);
CREATE INDEX idx_seq_log_full_number ON seq_log(full_number);
```

### 2.2 Message Format (RabbitMQ)

```json
{
  "message_id": "550e8400-e29b-41d4-a716-446655440000",
  "prefix": "SG",
  "counter": 123,
  "full_number": "SG000123",
  "generated_by": "api-service-01",
  "client_id": "erp-system",
  "generated_at": "2025-09-09T15:45:00Z",
  "published_at": "2025-09-09T15:45:00.123Z",
  "retry_count": 0,
  "correlation_id": "req-12345"
}
```

## 3. API Specifications

### 3.1 gRPC Service Definition

```protobuf
syntax = "proto3";

package sequential_id;
option go_package = "github.com/putram11/sequential-id-counter-service/pkg/proto";

service SequentialIDService {
  rpc GetNext(GetNextRequest) returns (GetNextResponse);
  rpc GetStatus(GetStatusRequest) returns (GetStatusResponse);
  rpc ResetCounter(ResetCounterRequest) returns (ResetCounterResponse);
  rpc UpdateConfig(UpdateConfigRequest) returns (UpdateConfigResponse);
  rpc GetConfig(GetConfigRequest) returns (GetConfigResponse);
}

message GetNextRequest {
  string prefix = 1;
  string client_id = 2;
  string generated_by = 3;
}

message GetNextResponse {
  string full_number = 1;
  int64 counter = 2;
  string prefix = 3;
}

message GetStatusRequest {
  string prefix = 1;
}

message GetStatusResponse {
  int64 current_counter = 1;
  int64 next_counter = 2;
  string prefix = 3;
  bool redis_healthy = 4;
}

message ResetCounterRequest {
  string prefix = 1;
  int64 set_to = 2;
  string reason = 3;
  string admin_user = 4;
}

message ResetCounterResponse {
  bool success = 1;
  string message = 2;
  int64 old_value = 3;
  int64 new_value = 4;
}

message UpdateConfigRequest {
  string prefix = 1;
  int32 padding_length = 2;
  string format_template = 3;
  string reset_rule = 4;
}

message UpdateConfigResponse {
  bool success = 1;
  string message = 2;
}

message GetConfigRequest {
  string prefix = 1;
}

message GetConfigResponse {
  string prefix = 1;
  int32 padding_length = 2;
  string format_template = 3;
  string reset_rule = 4;
  string last_reset_at = 5;
}
```

### 3.2 REST API Endpoints

```yaml
openapi: 3.0.0
info:
  title: Sequential ID Counter Service
  version: 1.0.0
  description: ERP-grade sequential ID generation service

paths:
  /api/v1/next/{prefix}:
    get:
      summary: Generate next sequential ID
      parameters:
        - name: prefix
          in: path
          required: true
          schema:
            type: string
        - name: client_id
          in: query
          schema:
            type: string
        - name: generated_by
          in: query
          schema:
            type: string
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  full_number:
                    type: string
                  counter:
                    type: integer
                  prefix:
                    type: string

  /api/v1/status/{prefix}:
    get:
      summary: Get counter status
      parameters:
        - name: prefix
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
          content:
            application/json:
              schema:
                type: object
                properties:
                  current_counter:
                    type: integer
                  next_counter:
                    type: integer
                  redis_healthy:
                    type: boolean

  /api/v1/reset/{prefix}:
    post:
      summary: Reset counter (admin only)
      security:
        - BearerAuth: []
      parameters:
        - name: prefix
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                set_to:
                  type: integer
                reason:
                  type: string
                admin_user:
                  type: string
      responses:
        '200':
          description: Success

  /api/v1/config/{prefix}:
    get:
      summary: Get prefix configuration
      parameters:
        - name: prefix
          in: path
          required: true
          schema:
            type: string
      responses:
        '200':
          description: Success
    
    post:
      summary: Update prefix configuration (admin only)
      security:
        - BearerAuth: []
      parameters:
        - name: prefix
          in: path
          required: true
          schema:
            type: string
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              properties:
                padding_length:
                  type: integer
                format_template:
                  type: string
                reset_rule:
                  type: string
      responses:
        '200':
          description: Success

components:
  securitySchemes:
    BearerAuth:
      type: http
      scheme: bearer
      bearerFormat: JWT
```

## 4. Startup & Recovery Logic

### 4.1 Service Initialization
1. **Database Connection**: Verify PostgreSQL connectivity
2. **Configuration Loading**: Load all prefix configurations from `seq_config`
3. **Redis Sync**: For each prefix, get `MAX(counter_value)` from `seq_log` or `seq_checkpoint`
4. **Counter Initialization**: Set Redis keys using `SET seq:<prefix> <max_value>`
5. **Health Checks**: Verify RabbitMQ and worker connectivity

### 4.2 Recovery Scenarios

#### Cold Start (Empty Redis)
```go
func syncRedisFromDatabase(prefix string) error {
    // Get max counter from audit log
    maxCounter := getMaxCounterFromDB(prefix)
    
    // Set Redis counter
    err := redisClient.Set(ctx, fmt.Sprintf("seq:%s", prefix), maxCounter, 0)
    if err != nil {
        return fmt.Errorf("failed to sync Redis: %w", err)
    }
    
    // Update checkpoint
    updateCheckpoint(prefix, maxCounter)
    return nil
}
```

#### Redis Failure Recovery
```go
func recoverFromRedisFailure() error {
    for _, prefix := range getAllPrefixes() {
        maxCounter := getMaxCounterFromDB(prefix)
        err := redisClient.Set(ctx, fmt.Sprintf("seq:%s", prefix), maxCounter, 0)
        if err != nil {
            log.Errorf("Failed to recover prefix %s: %v", prefix, err)
            continue
        }
        log.Infof("Recovered prefix %s with counter %d", prefix, maxCounter)
    }
    return nil
}
```

## 5. Consistency & Durability

### 5.1 Consistency Model
- **Redis INCR**: Guarantees atomic increment and uniqueness per prefix
- **Audit Trail**: PostgreSQL provides ACID guarantees for audit records
- **Eventual Consistency**: Audit logs are eventually consistent with Redis counters

### 5.2 Gap Handling
- **Acceptable Gaps**: If service crashes after Redis INCR but before RabbitMQ publish
- **Gap Detection**: Reconciliation jobs can identify and report gaps
- **Mitigation**: Use Redis AOF persistence with appropriate fsync policy

### 5.3 Idempotency
- **Message Deduplication**: Use `message_id` for idempotent processing
- **Database Constraints**: Unique constraint on `(prefix, counter_value)`

## 6. Scaling & Performance

### 6.1 Horizontal Scaling
- **API Services**: Stateless, scale behind load balancer
- **Workers**: Scale based on RabbitMQ queue depth
- **Database**: Read replicas for reporting queries

### 6.2 Performance Targets
- **Latency**: < 10ms for ID generation (99th percentile)
- **Throughput**: > 10,000 IDs/second per API instance
- **Availability**: 99.9% uptime

### 6.3 Caching Strategy
- **Redis**: Primary cache for active counters
- **Application Cache**: Configuration data caching
- **Connection Pooling**: Optimized connection management

## 7. Monitoring & Observability

### 7.1 Metrics (Prometheus)
```go
// Counter metrics
var (
    idsGenerated = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "sequential_ids_generated_total",
            Help: "Total number of sequential IDs generated",
        },
        []string{"prefix", "status"},
    )
    
    generationLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "sequential_id_generation_duration_seconds",
            Help: "Time taken to generate sequential ID",
        },
        []string{"prefix"},
    )
    
    redisOperations = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "redis_operations_total",
            Help: "Total Redis operations",
        },
        []string{"operation", "status"},
    )
    
    queueDepth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "rabbitmq_queue_depth",
            Help: "RabbitMQ queue depth",
        },
        []string{"queue"},
    )
)
```

### 7.2 Health Checks
```yaml
# Kubernetes health check configuration
livenessProbe:
  httpGet:
    path: /health/live
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10

readinessProbe:
  httpGet:
    path: /health/ready
    port: 8080
  initialDelaySeconds: 5
  periodSeconds: 5

# gRPC health check
grpc_health_probe -addr=localhost:9090
```

### 7.3 Alerting Rules
```yaml
# Prometheus alerting rules
groups:
  - name: sequential-id-service
    rules:
      - alert: HighIDGenerationLatency
        expr: histogram_quantile(0.99, sequential_id_generation_duration_seconds) > 0.1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High ID generation latency detected"
      
      - alert: RabbitMQQueueDepthHigh
        expr: rabbitmq_queue_depth > 10000
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "RabbitMQ queue depth is too high"
      
      - alert: RedisConnectionFailure
        expr: redis_operations_total{status="error"} > 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Redis connection failures detected"
```

## 8. Security

### 8.1 Authentication & Authorization
- **JWT Tokens**: Bearer token authentication for admin endpoints
- **RBAC**: Role-based access control for configuration changes
- **API Keys**: Service-to-service authentication

### 8.2 Network Security
- **TLS**: All connections encrypted (Redis, RabbitMQ, PostgreSQL)
- **VPC**: Isolated network environment
- **Firewall Rules**: Restrictive ingress/egress rules

### 8.3 Audit & Compliance
- **Access Logs**: All API requests logged with user context
- **Change Tracking**: Configuration changes tracked with user attribution
- **Data Retention**: Configurable audit log retention policies

## 9. Deployment Architecture

### 9.1 Kubernetes Deployment
```yaml
# API Service deployment
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sequential-id-api
spec:
  replicas: 3
  selector:
    matchLabels:
      app: sequential-id-api
  template:
    metadata:
      labels:
        app: sequential-id-api
    spec:
      containers:
      - name: api
        image: sequential-id-service:latest
        ports:
        - containerPort: 8080  # REST API
        - containerPort: 9090  # gRPC
        env:
        - name: REDIS_URL
          value: "redis://redis-cluster:6379"
        - name: RABBITMQ_URL
          value: "amqp://rabbitmq-cluster:5672"
        - name: DB_URL
          valueFrom:
            secretKeyRef:
              name: postgres-secret
              key: url
        resources:
          requests:
            memory: "256Mi"
            cpu: "250m"
          limits:
            memory: "512Mi"
            cpu: "500m"
```

### 9.2 Docker Compose (Development)
```yaml
version: '3.8'
services:
  api:
    build: .
    ports:
      - "8080:8080"
      - "9090:9090"
    environment:
      - REDIS_URL=redis://redis:6379
      - RABBITMQ_URL=amqp://rabbitmq:5672
      - DB_URL=postgres://user:pass@postgres:5432/seqdb
    depends_on:
      - redis
      - rabbitmq
      - postgres

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis_data:/data

  rabbitmq:
    image: rabbitmq:3-management
    environment:
      - RABBITMQ_DEFAULT_USER=admin
      - RABBITMQ_DEFAULT_PASS=admin
    ports:
      - "15672:15672"
    volumes:
      - rabbitmq_data:/var/lib/rabbitmq

  postgres:
    image: postgres:15
    environment:
      - POSTGRES_DB=seqdb
      - POSTGRES_USER=sequser
      - POSTGRES_PASSWORD=seqpass
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./migrations:/docker-entrypoint-initdb.d

  worker:
    build: .
    command: ./worker
    environment:
      - RABBITMQ_URL=amqp://rabbitmq:5672
      - DB_URL=postgres://user:pass@postgres:5432/seqdb
    depends_on:
      - rabbitmq
      - postgres

volumes:
  redis_data:
  rabbitmq_data:
  postgres_data:
```

## 10. Operational Procedures

### 10.1 Backup Strategy
```bash
# PostgreSQL backup
pg_dump -h postgres-host -U sequser seqdb > backup_$(date +%Y%m%d_%H%M%S).sql

# Redis backup (automatic RDB + AOF)
redis-cli --rdb dump.rdb
redis-cli BGREWRITEAOF

# Configuration backup
kubectl get configmap sequential-id-config -o yaml > config_backup.yaml
```

### 10.2 Recovery Procedures
```bash
# Database recovery
psql -h postgres-host -U sequser seqdb < backup_20250909_123000.sql

# Redis recovery (from AOF/RDB)
redis-server --appendonly yes --dir /data

# Reconciliation after recovery
./reconcile-tool --check-all-prefixes --fix-gaps
```

### 10.3 Monitoring Runbook
```bash
# Check service health
curl http://api-service:8080/health
grpc_health_probe -addr=api-service:9090

# Check queue depth
rabbitmqctl list_queues name messages

# Check Redis status
redis-cli info replication
redis-cli info persistence

# Check database lag
psql -c "SELECT application_name, state, sync_state FROM pg_stat_replication;"
```

## 11. Performance Tuning

### 11.1 Redis Configuration
```conf
# redis.conf optimizations
save 900 1
save 300 10
save 60 10000

appendonly yes
appendfsync everysec
auto-aof-rewrite-percentage 100
auto-aof-rewrite-min-size 64mb

maxmemory-policy noeviction
tcp-keepalive 60
timeout 0
```

### 11.2 PostgreSQL Tuning
```sql
-- postgresql.conf optimizations
shared_buffers = '256MB'
effective_cache_size = '1GB'
maintenance_work_mem = '64MB'
checkpoint_completion_target = 0.9
wal_buffers = '16MB'
default_statistics_target = 100
random_page_cost = 1.1
effective_io_concurrency = 200

-- Table partitioning for large audit logs
CREATE TABLE seq_log_2025_01 PARTITION OF seq_log
FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
```

### 11.3 RabbitMQ Configuration
```erlang
% rabbitmq.conf
vm_memory_high_watermark.relative = 0.4
disk_free_limit.absolute = 1GB
cluster_partition_handling = autoheal
queue_master_locator = min-masters
```

## 12. Testing Strategy

### 12.1 Unit Tests
- Redis operations (increment, get, set)
- Message formatting and parsing
- Configuration validation
- Error handling scenarios

### 12.2 Integration Tests
- End-to-end ID generation flow
- Database transaction integrity
- Message queue durability
- Service startup and recovery

### 12.3 Load Testing
- Concurrent ID generation
- Queue backpressure handling
- Database performance under load
- Redis cluster failover

### 12.4 Chaos Testing
- Redis node failures
- RabbitMQ node failures
- Database connection interruptions
- Network partitions

## 13. Migration & Deployment

### 13.1 Database Migrations
```sql
-- V001__initial_schema.sql
CREATE TABLE seq_config (...);
CREATE TABLE seq_log (...);
CREATE TABLE seq_checkpoint (...);

-- V002__add_indexes.sql
CREATE INDEX CONCURRENTLY idx_seq_log_prefix_counter ON seq_log(prefix, counter_value);

-- V003__add_partitioning.sql
-- Partition seq_log by month for better performance
```

### 13.2 Blue-Green Deployment
1. Deploy new version to green environment
2. Run health checks and smoke tests
3. Switch traffic gradually (canary deployment)
4. Monitor metrics and rollback if needed

### 13.3 Zero-Downtime Migration
- Database migrations run before code deployment
- Backward-compatible API changes
- Feature flags for new functionality

## 14. Cost Optimization

### 14.1 Resource Planning
- **Compute**: Auto-scaling based on CPU/memory usage
- **Storage**: Lifecycle policies for audit log retention
- **Network**: CDN for static assets, compression

### 14.2 Infrastructure Costs
- **Development**: Single-node deployments
- **Staging**: Smaller replicas, shared resources
- **Production**: Full HA setup with appropriate sizing

## 15. Compliance & Governance

### 15.1 Data Governance
- Data classification and handling procedures
- Retention policies for audit logs
- Privacy considerations (GDPR compliance)

### 15.2 Change Management
- Code review requirements
- Deployment approval workflows
- Rollback procedures

### 15.3 Documentation Standards
- API documentation (OpenAPI/gRPC docs)
- Operational runbooks
- Architecture decision records (ADRs)

---

This design provides a comprehensive foundation for implementing an enterprise-grade sequential ID counter service with strong consistency, auditability, and scalability characteristics suitable for ERP systems.
