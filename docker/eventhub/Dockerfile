FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY src/go.mod src/go.sum ./
RUN go mod download
COPY src/ .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /eventhub ./cmd/eventhub

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /eventhub /app/eventhub
EXPOSE 8080
ENTRYPOINT ["/app/eventhub"]
