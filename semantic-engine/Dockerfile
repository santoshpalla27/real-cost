# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/semantic-engine ./semantic-engine/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/semantic-engine /app/semantic-engine

EXPOSE 8082
ENTRYPOINT ["/app/semantic-engine"]
