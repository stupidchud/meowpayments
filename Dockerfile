FROM golang:1.25-alpine AS builder

WORKDIR /app

# Download dependencies first (cached layer)
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /meowpayments ./cmd/server

# ---

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

COPY --from=builder /meowpayments /meowpayments
COPY --from=builder /app/internal/store/migrations ./migrations

EXPOSE 8080

ENTRYPOINT ["/meowpayments"]
