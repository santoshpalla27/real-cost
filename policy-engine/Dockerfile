# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git
ENV GOPRIVATE=github.com/futuristic-iac/*
ENV GOSUMDB=off
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /bin/policy-engine ./policy-engine/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/policy-engine /app/policy-engine

EXPOSE 8086
ENTRYPOINT ["/app/policy-engine"]
