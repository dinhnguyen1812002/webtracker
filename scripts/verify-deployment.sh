#!/bin/bash

# Deployment verification script
set -e

echo "🚀 Verifying Uptime Monitor deployment..."

# Check if Docker is available
if ! command -v docker &> /dev/null; then
    echo "❌ Docker is not installed or not in PATH"
    exit 1
fi

# Check if Docker Compose is available
if ! docker compose version &> /dev/null; then
    echo "❌ Docker Compose is not available"
    exit 1
fi

echo "✅ Docker and Docker Compose are available"

# Validate Docker Compose configuration
echo "🔍 Validating Docker Compose configuration..."
if docker compose config > /dev/null; then
    echo "✅ Docker Compose configuration is valid"
else
    echo "❌ Docker Compose configuration is invalid"
    exit 1
fi

# Check if .env file exists
if [ ! -f .env ]; then
    echo "⚠️  .env file not found, creating from template..."
    cp .env.example .env
    echo "✅ Created .env file from template"
    echo "📝 Please edit .env file with your configuration"
fi

# Build the Docker image
echo "🔨 Building Docker image..."
if docker build -t uptime-monitor . > /dev/null; then
    echo "✅ Docker image built successfully"
else
    echo "❌ Failed to build Docker image"
    exit 1
fi

# Test migration tool
echo "🔧 Testing migration tool..."
if go build -o scripts/migrate ./scripts/migrate.go; then
    echo "✅ Migration tool built successfully"
    ./scripts/migrate -help > /dev/null
    echo "✅ Migration tool is working"
else
    echo "❌ Failed to build migration tool"
    exit 1
fi

# Check if ports are available
echo "🔍 Checking port availability..."
for port in 8080 5432 6379; do
    if lsof -i :$port > /dev/null 2>&1; then
        echo "⚠️  Port $port is already in use"
    else
        echo "✅ Port $port is available"
    fi
done

echo ""
echo "🎉 Deployment verification completed!"
echo ""
echo "Next steps:"
echo "1. Edit .env file with your configuration"
echo "2. Run: docker compose up -d"
echo "3. Check status: docker compose ps"
echo "4. View logs: docker compose logs -f uptime-monitor-app"
echo "5. Access application: http://localhost:8080"
echo ""
echo "For production deployment:"
echo "docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d"
