FROM --platform=$BUILDPLATFORM node:22-alpine AS frontend-builder

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS backend-builder

ARG TARGETARCH

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build -o /clawbot-gateway .

FROM alpine:3.19

RUN apk add --no-cache tzdata ca-certificates
RUN cp /usr/share/zoneinfo/Asia/Shanghai /etc/localtime && \
    echo "Asia/Shanghai" > /etc/timezone

WORKDIR /app

COPY --from=backend-builder /clawbot-gateway .
COPY --from=frontend-builder /build/web/dist/ web/dist/

VOLUME ["/app/data"]
EXPOSE 6798

ENV CLAWBOT_DB_PATH=data/clawbot.db
ENV CLAWBOT_HOST=0.0.0.0
ENV CLAWBOT_PORT=6798
ENV CLAWBOT_LOG_LEVEL=info

CMD ["./clawbot-gateway"]