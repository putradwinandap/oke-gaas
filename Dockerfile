FROM golang:1.26.4@sha256:f96cc555eb8db430159a3aa6797cd5bae561945b7b0fe7d0e284c63a3b291609 AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/oke-gaas ./cmd/server

FROM alpine:3.22.6@sha256:abd29214470819ed7667c87c1ceebc89aae766453a5b3cc09e8a52b9f796fd5a

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
