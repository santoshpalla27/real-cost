FROM golang:1.23-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o app ./cmd/estimation

FROM gcr.io/distroless/base-debian12
COPY --from=build /app/app /app/app
EXPOSE 8085
ENTRYPOINT ["/app/app"]
