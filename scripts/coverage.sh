#!/bin/bash

# Run tests with coverage
docker-compose -f .devcontainer/docker-compose.yml exec app go test ./... -coverprofile=coverage.out

# Display coverage report in terminal
docker-compose -f .devcontainer/docker-compose.yml exec app go tool cover -func=coverage.out

# Generate HTML coverage report
docker-compose -f .devcontainer/docker-compose.yml exec app go tool cover -html=coverage.out -o coverage.html

echo "Coverage report generated at coverage.html" 