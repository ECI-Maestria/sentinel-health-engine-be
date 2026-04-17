FROM golang:1.23-alpine AS builder

WORKDIR /build

# Copy workspace and all modules (go.work references all three services)
COPY go.work go.work.sum* ./
COPY shared/ ./shared/
COPY services/telemetry-service/ ./services/telemetry-service/
COPY services/health-rules-service/ ./services/health-rules-service/
COPY services/alerts-service/ ./services/alerts-service/
COPY services/user-service/ ./services/user-service/
COPY services/analytics-service/ ./services/analytics-service/
COPY services/calendar-service/ ./services/calendar-service/

WORKDIR /build/services/health-rules-service
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-w -s" -o /app/server ./cmd/server/...

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /app/server /server
EXPOSE 8080
ENTRYPOINT ["/server"]
