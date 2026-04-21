FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o zai-proxy .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/zai-proxy .

EXPOSE 8000

CMD ["./zai-proxy"]
