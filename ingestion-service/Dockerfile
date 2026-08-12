FROM golang:1.24-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/ingestion-service ./cmd/server

FROM alpine:3.19

RUN apk add --no-cache ca-certificates && \
    adduser -D -H appuser
USER appuser

COPY --from=builder /out/ingestion-service /usr/local/bin/ingestion-service

EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/ingestion-service"]
