# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/budget-service ./budget-service/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/budget-service /app/budget-service

EXPOSE 8087
ENTRYPOINT ["/app/budget-service"]
