# Sequential ID Counter Service

Enterprise-grade sequential ID counter service providing atomic, scalable, and auditable ID generation with Redis, RabbitMQ, and PostgreSQL.

## Features

- **Atomic ID Generation**: Redis-based counters ensure uniqueness
- **Audit Trail**: Complete PostgreSQL-based audit logging
- **High Availability**: Clustered deployment with automatic failover
- **Dual APIs**: Both REST and gRPC interfaces
- **Scalable**: Horizontal scaling with event-driven architecture
- **ERP-Ready**: Designed for enterprise resource planning systems

## Quick Start

### Development Environment

```bash
# Clone and setup
git clone https://github.com/putram11/sequential-id-counter-service.git
cd sequential-id-counter-service

# Start with Docker Compose
docker-compose up -d

# Initialize database
make migrate-up

# Run tests
make test

# Build and run
make build
./bin/sequential-id-service
```

### Usage Examples

#### REST API
```bash
# Generate next ID
curl "http://localhost:8080/api/v1/next/SG?client_id=erp-system"
# Response: {"full_number":"SG000001","counter":1,"prefix":"SG"}

# Check status
curl "http://localhost:8080/api/v1/status/SG"
# Response: {"current_counter":1,"next_counter":2,"redis_healthy":true}
```

#### gRPC Client
```go
conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
client := pb.NewSequentialIDServiceClient(conn)

resp, err := client.GetNext(ctx, &pb.GetNextRequest{
    Prefix:      "INV",
    ClientId:    "erp-system",
    GeneratedBy: "user123",
})
fmt.Printf("Generated: %s\n", resp.FullNumber)
```

## Architecture

```
[Clients] → [Load Balancer] → [API Services] → [Redis Cluster]
                                    ↓
                               [RabbitMQ] → [Workers] → [PostgreSQL]
```

## Configuration

### Environment Variables

```bash
# Service Configuration
PORT=8080
GRPC_PORT=9090
LOG_LEVEL=info
ENVIRONMENT=development

# Redis Configuration
REDIS_URL=redis://localhost:6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_CLUSTER_MODE=false

# RabbitMQ Configuration
RABBITMQ_URL=amqp://guest:guest@localhost:5672/
RABBITMQ_EXCHANGE=seq_exchange
RABBITMQ_QUEUE=seq_log_queue

# PostgreSQL Configuration
DB_URL=postgres://sequser:seqpass@localhost:5432/seqdb?sslmode=disable
DB_MAX_OPEN_CONNS=25
DB_MAX_IDLE_CONNS=5

# Security
JWT_SECRET=your-jwt-secret-key
API_KEY=your-api-key

# Monitoring
METRICS_PORT=2112
HEALTH_CHECK_PORT=8081
```

### Prefix Configuration

```sql
INSERT INTO seq_config (prefix, padding_length, format_template, reset_rule) VALUES 
('SG', 6, '%s%06d', 'never'),
('INV', 4, 'INV%d-%04d', 'yearly'),
('PO', 8, '%s%08d', 'monthly');
```

## API Reference

See [API Documentation](./docs/api.md) for complete REST and gRPC API specifications.

## Deployment

### Kubernetes
```bash
# Deploy to Kubernetes
kubectl apply -f k8s/

# Check status
kubectl get pods -l app=sequential-id-service
```

### Docker Compose
```bash
# Production deployment
docker-compose -f docker-compose.prod.yml up -d
```

## Monitoring

- **Metrics**: Prometheus metrics on `:2112/metrics`
- **Health**: Health checks on `:8081/health`
- **Tracing**: OpenTelemetry integration
- **Logging**: Structured JSON logging

### Key Metrics
- `sequential_ids_generated_total`
- `sequential_id_generation_duration_seconds`
- `redis_operations_total`
- `rabbitmq_queue_depth`

## Security

- JWT-based authentication for admin endpoints
- TLS encryption for all external communications
- RBAC for configuration management
- Audit logging for all operations

## Development

### Prerequisites
- Go 1.21+
- Docker & Docker Compose
- PostgreSQL 15+
- Redis 7+
- RabbitMQ 3.12+

### Project Structure
```
├── cmd/                    # Application entry points
├── internal/              # Private application code
│   ├── api/              # API handlers (REST/gRPC)
│   ├── service/          # Business logic
│   ├── repository/       # Data access layer
│   └── worker/           # Background workers
├── pkg/                  # Public packages
├── migrations/           # Database migrations
├── scripts/              # Utility scripts
├── k8s/                  # Kubernetes manifests
└── docs/                 # Documentation
```

### Testing
```bash
# Unit tests
make test

# Integration tests
make test-integration

# Load tests
make test-load

# Coverage report
make coverage
```

## Performance

- **Latency**: <10ms (99th percentile)
- **Throughput**: >10,000 IDs/second per instance
- **Availability**: 99.9% uptime target

## Support

- **Documentation**: [/docs](./docs/)
- **Issues**: [GitHub Issues](https://github.com/putram11/sequential-id-counter-service/issues)
- **Discussions**: [GitHub Discussions](https://github.com/putram11/sequential-id-counter-service/discussions)

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

See [CONTRIBUTING.md](CONTRIBUTING.md) for detailed guidelines.