# Build stage
FROM rust:1.77-alpine AS builder

RUN apk add --no-cache musl-dev

WORKDIR /app

COPY Cargo.toml Cargo.lock* ./
RUN mkdir src && echo "fn main() {}" > src/main.rs
RUN cargo build --release 2>/dev/null; exit 0
RUN rm -rf src

COPY . .
RUN cargo build --release

# Runtime stage
FROM alpine:3.19

RUN apk add --no-cache ca-certificates
RUN adduser -D -g '' valtag

COPY --from=builder /app/target/release/valtag-mcp-server /usr/local/bin/

USER valtag

CMD ["valtag-mcp-server"]
