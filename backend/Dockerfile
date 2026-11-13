FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/server ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates wget
WORKDIR /app
COPY --from=build /out/server /app/server
ENV PORT=8080 \
    DATABASE_PATH=/data/geofence.db \
    SEED_DATA=true
VOLUME ["/data"]
EXPOSE 8080
ENTRYPOINT ["/app/server"]
