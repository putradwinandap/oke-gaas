FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/oke-gaas ./cmd/server

FROM alpine:3.22

RUN apk add --no-cache ca-certificates curl \
    && addgroup -S oke-gaas \
    && adduser -S -G oke-gaas oke-gaas

WORKDIR /app
COPY --from=builder /out/oke-gaas /usr/local/bin/oke-gaas

USER oke-gaas:oke-gaas

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=10s --retries=3 \
  CMD curl --fail --silent http://127.0.0.1:8080/health >/dev/null || exit 1

ENTRYPOINT ["/usr/local/bin/oke-gaas"]
