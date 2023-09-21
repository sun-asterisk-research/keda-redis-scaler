FROM golang:1.21.1-alpine3.18 AS builder

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN go build -o /app/bin/keda-redis-scaler .

FROM alpine:3.18

WORKDIR /app

COPY --from=builder /app/bin/keda-redis-scaler /usr/local/bin

CMD ["keda-redis-scaler"]
