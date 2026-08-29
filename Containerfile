FROM oven/bun:latest AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package.json frontend/bun.lock* ./
RUN bun install
COPY frontend/ .
RUN bun run build

FROM golang:alpine AS backend-builder
WORKDIR /app

RUN apk add --no-cache git

COPY . .

COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist

RUN go mod tidy
RUN CGO_ENABLED=0 GOOS=linux go build -o litekit .

FROM alpine:latest
WORKDIR /app
RUN apk add --no-cache ca-certificates
COPY --from=backend-builder /app/litekit /app/litekit

EXPOSE 8080
CMD ["/app/litekit"]