FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/mini-drop-api ./apps/api-server

FROM golang:1.25-bookworm

WORKDIR /app
COPY --from=builder /out/mini-drop-api /app/mini-drop-api

ENV MINIDROP_API_ADDR=0.0.0.0:8080
ENV MINIDROP_DATA_DIR=/app/data
ENV MINIDROP_DB_DRIVER=sqlite
ENV MINIDROP_DB_PATH=/app/data/mini-drop.db
ENV MINIDROP_ARTIFACT_DIR=/app/artifacts
ENV MINIDROP_ALLOWED_ORIGIN=http://localhost,http://127.0.0.1

EXPOSE 8080
CMD ["/app/mini-drop-api"]
