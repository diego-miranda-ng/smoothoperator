# Go development container
FROM golang:1.25

# Install basic tools
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Create app directory
WORKDIR /app

# Default command (overridden by devcontainer)
CMD ["sleep", "infinity"]