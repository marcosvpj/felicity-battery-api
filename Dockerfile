# Stage 1: Build static binary
FROM golang:1.22-alpine AS builder
WORKDIR /src
COPY go.mod .
COPY *.go .
COPY dashboard.html .
RUN CGO_ENABLED=0 GOOS=linux go build -o /felicity-battery .

# Stage 2: Minimal runtime image (~10 MB)
FROM alpine:3.19
RUN apk add --no-cache ca-certificates tzdata wget
WORKDIR /app
COPY --from=builder /felicity-battery .
RUN mkdir -p /data
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/felicity-battery"]
CMD ["-serve", ":8080", "-history", "/data/battery.jsonl"]
