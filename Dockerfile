FROM golang:1.22-bookworm AS builder

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download && go mod verify

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o /go/bin/app ./cmd/web

FROM debian:12.5-slim

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

WORKDIR /api/

COPY --from=builder /go/bin/app ./bin/app
COPY --from=builder /go/src/app/.env .

CMD ["/api/bin/app", "-env", "/api/.env"]

LABEL Name=user-authentication-service Version=0.0.1

EXPOSE 3000

HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:3000/health || exit 1

