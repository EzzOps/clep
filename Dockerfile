FROM golang:1.21-alpine AS builder

WORKDIR /build

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gateway ./gateway/...

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
ENV TZ=UTC

WORKDIR /app

COPY --from=builder /build/gateway .

EXPOSE 8080

ENTRYPOINT ["/app/gateway"]
