FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o ingestion_app ./cmd/gateway

# ---------
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add tzdata

COPY --from=builder /app/ingestion_app .

CMD ["./ingestion_app"]