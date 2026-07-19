FROM golang:1.22-alpine AS builder

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
COPY config.yaml .
COPY web/ web/

EXPOSE 8080

VOLUME ["/app/data"]

CMD ["./clawbot-gateway", "config.yaml"]
