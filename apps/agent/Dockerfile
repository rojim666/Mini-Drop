FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/mini-drop-agent ./apps/agent

FROM python:3.14-slim
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=builder /out/mini-drop-agent /app/mini-drop-agent
COPY apps/analyzer /app/analyzer

ENV MINIDROP_API_BASE_URL=http://api-server:8080
ENV MINIDROP_AGENT_ID=agt_compose
ENV MINIDROP_AGENT_HOSTNAME=compose-agent
ENV MINIDROP_AGENT_IP=demo-target
ENV MINIDROP_AGENT_VERSION=0.1.0
ENV MINIDROP_PYTHON_BIN=python
ENV MINIDROP_ANALYZER_SCRIPT=/app/analyzer/main.py
ENV MINIDROP_ARTIFACT_DIR=/app/artifacts

CMD ["/app/mini-drop-agent"]
