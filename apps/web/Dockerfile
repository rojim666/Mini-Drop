FROM oven/bun:1.3.14 AS builder
WORKDIR /app

COPY apps/web/package*.json ./
RUN bun install --frozen-lockfile

COPY apps/web ./
ARG VITE_API_BASE_URL=http://localhost:8080
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN bun run build

FROM golang:1.25-bookworm
WORKDIR /usr/share/nginx/html
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
CMD ["python3", "-m", "http.server", "80", "--bind", "0.0.0.0", "--directory", "/usr/share/nginx/html"]
