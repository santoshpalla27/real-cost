# ---------- BUILD STAGE ----------
FROM golang:1.23-alpine AS builder

WORKDIR /build
RUN apk add --no-cache git
ENV GOPRIVATE=github.com/futuristic-iac/*
ENV GOSUMDB=off
COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -v -o /bin/estimation-core ./estimation-core/main.go

# ---------- RUNTIME STAGE ----------
FROM gcr.io/distroless/static-debian12

WORKDIR /app
COPY --from=builder /bin/estimation-core /app/estimation-core

EXPOSE 8085
ENTRYPOINT ["/app/estimation-core"]
