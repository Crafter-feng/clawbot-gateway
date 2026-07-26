FROM golang:1.25-alpine AS builder

WORKDIR /build

# 依赖缓存
COPY go.mod go.sum ./
RUN go mod download

# 构建
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /clawbot-gateway .

# ── 运行镜像 ──
FROM alpine:3.19

RUN apk add --no-cache tzdata ca-certificates
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=builder /clawbot-gateway .
COPY web/ web/

# 数据目录
VOLUME ["/app/data"]

EXPOSE 8080

# 环境变量（可通过 docker-compose 或 -e 传入）
ENV CLAWBOT_DB_PATH=data/clawbot.db
ENV CLAWBOT_HOST=0.0.0.0
ENV CLAWBOT_PORT=8080
ENV CLAWBOT_LOG_LEVEL=info

CMD ["./clawbot-gateway"]
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser
