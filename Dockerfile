FROM golang:1.20.0-alpine3.16 as builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod .
COPY go.sum .

# Get dependancies - will also be cached if we won't change mod/sum
RUN go mod download

# Build
COPY . .
RUN go build .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=builder /app/prometheus-hsdp-metrics-exporter /app

EXPOSE 8889
CMD ["/app/prometheus-hsdp-metrics-exporter"]
