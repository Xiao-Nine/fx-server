# Build stage
FROM golang:1.25-alpine AS builder

# Set Go proxy to Aliyun mirror for faster downloads in China
ENV GOPROXY=GOPROXY=https://goproxy.cn,direct
# Install build dependencies for CGO/SQLite
RUN apk add --no-cache gcc musl-dev sqlite-dev

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application with CGO enabled for SQLite
RUN CGO_ENABLED=1 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o fx-server .

# Runtime stage - minimal alpine image
FROM alpine:latest

# Install runtime dependencies
RUN apk --no-cache add ca-certificates sqlite-libs

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/fx-server .

# Create data directories
RUN mkdir -p /app/data/uploads

# Expose port
EXPOSE 8080

# Run the application
CMD ["./fx-server"]