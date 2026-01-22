FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o fiac-server ./cmd/server

FROM gcr.io/distroless/base-debian12
WORKDIR /app
COPY --from=build /app/fiac-server /app/fiac-server
COPY --from=build /app/policies /app/policies
ENV PORT=8080
ENV POLICIES_DIR=/app/policies
EXPOSE 8080
ENTRYPOINT ["/app/fiac-server"]
