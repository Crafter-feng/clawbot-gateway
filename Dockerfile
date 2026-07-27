FROM node:20-alpine AS frontend-builder

WORKDIR /build/web
COPY web/package.json web/pnpm-lock.yaml* ./
RUN corepack enable && pnpm install --frozen-lockfile
COPY web/ .
RUN npm run build

FROM golang:1.22-alpine AS backend-builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /clawbot-gateway .

FROM scratch

COPY --from=backend-builder /etc/ssl/cert.pem /etc/ssl/cert.pem
COPY --from=backend-builder /usr/share/zoneinfo/Asia/Shanghai /usr/share/zoneinfo/Asia/Shanghai

COPY --from=backend-builder /clawbot-gateway /app/clawbot-gateway
COPY --from=frontend-builder /build/web/dist/ /app/web/dist/

WORKDIR /app
VOLUME ["/app/data"]
EXPOSE 8080

ENV CLAWBOT_DB_PATH=data/clawbot.db
ENV CLAWBOT_HOST=0.0.0.0
ENV CLAWBOT_PORT=8080
ENV CLAWBOT_LOG_LEVEL=info

CMD ["/app/clawbot-gateway"]