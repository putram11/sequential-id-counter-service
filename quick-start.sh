#!/bin/bash

# Sequential ID Counter Service - Quick Start Script
# This script helps you get started with the service quickly

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
print_header() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "  Sequential ID Counter Service - Quick Start"
    echo "=================================================="
    echo -e "${NC}"
}

print_step() {
    echo -e "${GREEN}[STEP]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_requirements() {
    print_step "Checking requirements..."
    
    # Check Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker is not installed. Please install Docker first."
        exit 1
    fi
    
    # Check Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose is not installed. Please install Docker Compose first."
        exit 1
    fi
    
    # Check if ports are available
    if netstat -tuln | grep -q ":8080 "; then
        print_warning "Port 8080 is already in use. The API service might conflict."
    fi
    
    if netstat -tuln | grep -q ":5432 "; then
        print_warning "Port 5432 is already in use. PostgreSQL might conflict."
    fi
    
    print_step "Requirements check completed."
}

start_services() {
    print_step "Starting services with Docker Compose..."
    
    # Start development environment
    docker-compose --profile dev up -d
    
    print_step "Waiting for services to be ready..."
    sleep 30
    
    # Check service health
    check_health
}

check_health() {
    print_step "Checking service health..."
    
    # Check API health
    if curl -f http://localhost:8081/health &> /dev/null; then
        echo -e "${GREEN}✓${NC} API service is healthy"
    else
        echo -e "${RED}✗${NC} API service is not responding"
    fi
    
    # Check Redis
    if docker-compose exec redis redis-cli ping &> /dev/null; then
        echo -e "${GREEN}✓${NC} Redis is healthy"
    else
        echo -e "${RED}✗${NC} Redis is not responding"
    fi
    
    # Check RabbitMQ
    if curl -f http://localhost:15672 &> /dev/null; then
        echo -e "${GREEN}✓${NC} RabbitMQ management is accessible"
    else
        echo -e "${RED}✗${NC} RabbitMQ is not responding"
    fi
    
    # Check PostgreSQL
    if docker-compose exec postgres pg_isready -U sequser &> /dev/null; then
        echo -e "${GREEN}✓${NC} PostgreSQL is healthy"
    else
        echo -e "${RED}✗${NC} PostgreSQL is not responding"
    fi
}

run_examples() {
    print_step "Running API examples..."
    
    echo "1. Generating sequential IDs:"
    
    # Generate some IDs
    for i in {1..5}; do
        response=$(curl -s "http://localhost:8080/api/v1/next/SG?client_id=demo")
        echo "   Generated: $(echo $response | jq -r '.full_number // "Error"')"
        sleep 0.5
    done
    
    echo ""
    echo "2. Checking status:"
    status=$(curl -s "http://localhost:8080/api/v1/status/SG")
    echo "   Current counter: $(echo $status | jq -r '.current_counter // "Error"')"
    echo "   Next counter: $(echo $status | jq -r '.next_counter // "Error"')"
    
    echo ""
    echo "3. Testing different prefix:"
    for prefix in INV PO QUO; do
        response=$(curl -s "http://localhost:8080/api/v1/next/$prefix?client_id=demo")
        echo "   $prefix: $(echo $response | jq -r '.full_number // "Error"')"
        sleep 0.2
    done
}

show_access_info() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "  Service Access Information"
    echo "=================================================="
    echo -e "${NC}"
    
    echo "🚀 API Service:"
    echo "   REST API:     http://localhost:8080/api/v1/"
    echo "   gRPC API:     localhost:9090"
    echo "   Health Check: http://localhost:8081/health"
    echo "   Metrics:      http://localhost:2112/metrics"
    echo ""
    
    echo "🗄️  Databases:"
    echo "   PostgreSQL:   localhost:5432 (sequser/seqpass)"
    echo "   Redis:        localhost:6379"
    echo ""
    
    echo "📊 Monitoring:"
    echo "   Prometheus:   http://localhost:9090"
    echo "   Grafana:      http://localhost:3000 (admin/admin)"
    echo ""
    
    echo "🛠️  Management:"
    echo "   RabbitMQ UI:  http://localhost:15672 (guest/guest)"
    echo "   Adminer:      http://localhost:8083"
    echo "   Redis Cmd:    http://localhost:8082"
    echo ""
    
    echo "📖 Documentation:"
    echo "   API Docs:     http://localhost:8080/swagger/"
    echo "   README:       ./README.md"
    echo "   Design Doc:   ./DESIGN.md"
}

show_usage_examples() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "  Usage Examples"
    echo "=================================================="
    echo -e "${NC}"
    
    echo "📝 REST API Examples:"
    echo "   # Generate next ID"
    echo "   curl \"http://localhost:8080/api/v1/next/SG?client_id=my-app\""
    echo ""
    echo "   # Check status"
    echo "   curl \"http://localhost:8080/api/v1/status/SG\""
    echo ""
    echo "   # Get configuration"
    echo "   curl \"http://localhost:8080/api/v1/config/SG\""
    echo ""
    
    echo "🔧 gRPC Examples:"
    echo "   # Install grpcurl first: go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest"
    echo "   grpcurl -plaintext -d '{\"prefix\":\"SG\",\"client_id\":\"demo\"}' \\"
    echo "     localhost:9090 sequential_id.SequentialIDService/GetNext"
    echo ""
    
    echo "🐍 Python Example:"
    echo "   import requests"
    echo "   resp = requests.get('http://localhost:8080/api/v1/next/SG?client_id=python')"
    echo "   print(resp.json()['full_number'])"
    echo ""
    
    echo "📊 Monitoring:"
    echo "   # View metrics"
    echo "   curl http://localhost:2112/metrics"
    echo ""
    echo "   # Check health"
    echo "   curl http://localhost:8081/health"
}

cleanup() {
    print_step "Cleaning up..."
    docker-compose --profile dev down -v
    print_step "Cleanup completed."
}

main() {
    print_header
    
    case "${1:-start}" in
        "start")
            check_requirements
            start_services
            run_examples
            show_access_info
            show_usage_examples
            ;;
        "stop")
            print_step "Stopping services..."
            docker-compose --profile dev down
            print_step "Services stopped."
            ;;
        "restart")
            print_step "Restarting services..."
            docker-compose --profile dev down
            docker-compose --profile dev up -d
            sleep 20
            check_health
            ;;
        "logs")
            print_step "Showing service logs..."
            docker-compose logs -f
            ;;
        "health")
            check_health
            ;;
        "examples")
            run_examples
            ;;
        "cleanup")
            cleanup
            ;;
        "help"|"-h"|"--help")
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  start     Start all services (default)"
            echo "  stop      Stop all services"
            echo "  restart   Restart all services"
            echo "  logs      Show service logs"
            echo "  health    Check service health"
            echo "  examples  Run API examples"
            echo "  cleanup   Stop and remove all containers/volumes"
            echo "  help      Show this help message"
            ;;
        *)
            print_error "Unknown command: $1"
            echo "Use '$0 help' for usage information."
            exit 1
            ;;
    esac
}

# Trap to handle Ctrl+C
trap 'echo -e "\n${YELLOW}Interrupted by user${NC}"; exit 130' INT

# Run main function
main "$@"
