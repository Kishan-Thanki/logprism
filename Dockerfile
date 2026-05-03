# Stage 1: Build
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o logprism ./cmd/logprism

# Stage 2: Final Image
FROM alpine:latest
WORKDIR /root/
COPY --from=builder /app/logprism .

# Usage Examples:
# 1. Basic usage: 
#    cat logs.json | docker run -i logprism
# 2. Forced colors (useful for pipes):
#    cat logs.json | docker run -i logprism -color
# 3. Filtering and Pretty-Printing:
#    cat logs.json | docker run -i logprism -filter level=ERROR -pretty

ENTRYPOINT ["./logprism"]
