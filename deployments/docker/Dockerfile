# =============================================================================
# Stage 1: Builder
# =============================================================================
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s -X main.version=${VERSION:-dev}" \
    -o /build/statuspage \
    ./cmd/statuspage

# =============================================================================
# Stage 2: Runtime
# =============================================================================
FROM alpine:3.19 AS runtime

RUN apk add --no-cache ca-certificates tzdata

RUN addgroup -g 1000 -S statuspage && \
    adduser -u 1000 -S statuspage -G statuspage

WORKDIR /app

COPY --from=builder /build/statuspage .

COPY --from=builder /build/migrations ./migrations

COPY --from=builder /build/api ./api

RUN chown -R statuspage:statuspage /app

USER statuspage

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/healthz || exit 1

ENTRYPOINT ["/app/statuspage"]
