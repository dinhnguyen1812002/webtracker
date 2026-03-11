#!/bin/bash

# Build script for dashboard with TailwindCSS
# This script generates the templ templates and builds the application

set -e

echo "Building Uptime Monitor Dashboard..."

# Ensure templ is installed
if ! command -v templ &> /dev/null; then
    echo "Installing templ..."
    go install github.com/a-h/templ/cmd/templ@latest
fi

# Generate templ templates
echo "Generating templ templates..."
templ generate interface/http/templates/

# Build the application
echo "Building application..."
go build -o bin/uptime-monitor ./cmd/main.go

echo "Dashboard build complete!"
echo ""
echo "TailwindCSS Configuration:"
echo "- Using TailwindCSS via CDN for rapid development"
echo "- Custom color palette configured for status indicators"
echo "- Responsive design with mobile-first approach"
echo "- Real-time updates via WebSocket integration"
echo ""
echo "To run the application:"
echo "  ./bin/uptime-monitor -config config.yaml"