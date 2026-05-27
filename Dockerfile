# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /src
RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /kanban ./cmd/kanban

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app

COPY --from=builder /kanban /app/kanban
COPY server.yaml /app/server.yaml
COPY web /app/web

RUN mkdir -p /app/data

ENV KANBAN_CONFIG=/app/server.yaml

EXPOSE 61333
VOLUME ["/app/data"]

ENTRYPOINT ["/app/kanban", "-config", "/app/server.yaml"]
