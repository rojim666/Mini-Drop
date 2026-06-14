FROM node:24-bookworm-slim AS builder
WORKDIR /app

COPY apps/web/package*.json ./
RUN npm ci

COPY apps/web ./
ARG VITE_API_BASE_URL=http://localhost:8080
ENV VITE_API_BASE_URL=${VITE_API_BASE_URL}
RUN npm run build

FROM nginx:1.29-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
EXPOSE 80
