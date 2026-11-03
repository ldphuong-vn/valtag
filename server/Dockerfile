# Build stage — builder Go version must match go.mod (go 1.26.x):
# GOTOOLCHAIN=local makes the build deterministic instead of downloading a
# toolchain from the module proxy at build time (GitHub runners share IPs
# and those downloads get rate-limited).
FROM golang:1.27-alpine AS builder

ENV GOTOOLCHAIN=local

RUN apk add --no-cache git ca-certificates

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-migrate ./cmd/migrate
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-seed ./cmd/seed
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /valtag-rotate-master-key ./cmd/rotate-master-key

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata
RUN adduser -D -g '' valtag

COPY --from=builder /valtag-server /usr/local/bin/valtag-server
COPY --from=builder /valtag-migrate /usr/local/bin/valtag-migrate
COPY --from=builder /valtag-seed /usr/local/bin/valtag-seed
COPY --from=builder /valtag-rotate-master-key /usr/local/bin/valtag-rotate-master-key

USER valtag

EXPOSE 8080

CMD ["valtag-server"]
