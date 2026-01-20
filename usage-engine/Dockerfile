# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/usage-engine ./usage-engine/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/usage-engine /app/usage-engine

EXPOSE 8083
ENTRYPOINT ["/app/usage-engine"]
