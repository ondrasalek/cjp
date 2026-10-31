# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS go-builder
ENV GOTOOLCHAIN=auto
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ ./cmd/
COPY internal/ ./internal/

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cjp-api ./cmd/cjp-api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/cjp-etl ./cmd/cjp-etl

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata

COPY --from=go-builder /out/cjp-api /usr/local/bin/cjp-api
COPY --from=go-builder /out/cjp-etl /usr/local/bin/cjp-etl
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh && mkdir -p /data

VOLUME ["/data"]
EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -qO- http://127.0.0.1:8080/healthz >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
