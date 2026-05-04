FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
ARG VERSION=dev
RUN go build -ldflags "-s -w -X main.version=${VERSION}" -o logprism ./cmd/logprism

FROM alpine:latest
RUN addgroup -S logprism && adduser -S logprism -G logprism
USER logprism
WORKDIR /home/logprism
COPY --from=builder /app/logprism .

ENTRYPOINT ["./logprism"]
