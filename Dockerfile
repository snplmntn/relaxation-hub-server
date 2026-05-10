# syntax=docker/dockerfile:1

FROM golang:1.25.4-alpine AS builder
WORKDIR /app
RUN apk add --no-cache ca-certificates build-base
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o server ./cmd/server

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates wget curl \
    && addgroup -S app \
    && adduser -S app -G app
COPY --from=builder --chown=app:app /app/server /app/server
ENV PORT=8080
EXPOSE 8080
USER app
CMD ["/app/server"]
