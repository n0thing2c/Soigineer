FROM golang:1.26.4-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o alert_app ./cmd/alert

# ---------
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add tzdata

COPY --from=builder /app/alert_app .

CMD ["./alert_app"]
