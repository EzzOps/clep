FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY gateway/go.mod gateway/go.sum ./
RUN go mod download
COPY gateway/*.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o gateway .
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata
ENV TZ=UTC
WORKDIR /app
COPY --from=builder /build/gateway .
EXPOSE 8080
ENTRYPOINT ["/app/gateway"]
