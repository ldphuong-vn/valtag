# Build stage
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod ./
RUN go mod download || true

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-seed ./cmd/seed

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' valtag

COPY --from=builder /valtag-server /usr/local/bin/valtag-server
COPY --from=builder /valtag-migrate /usr/local/bin/valtag-migrate
COPY --from=builder /valtag-seed /usr/local/bin/valtag-seed

USER valtag

EXPOSE 8080

CMD ["valtag-server"]
