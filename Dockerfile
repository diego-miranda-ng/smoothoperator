FROM golang:1.25

WORKDIR /app

# Only install delve for debugging support
RUN go install github.com/go-delve/delve/cmd/dlv@latest

# Copy go.mod first
COPY go.mod ./

# Initialize go.sum (if it doesn't exist)
RUN go mod download && \
    go mod tidy

# Copy the rest of the code
COPY . .

# Default command to keep container running
CMD ["tail", "-f", "/dev/null"] 