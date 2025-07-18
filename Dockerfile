# 构建阶段
FROM golang:alpine AS builder

WORKDIR /app

# 安装必要的构建工具和依赖
RUN apk add --no-cache build-base git

# 复制依赖文件并下载模块
COPY go.mod go.sum ./
RUN go mod download

# 复制所有源码
COPY . .

# 构建应用（指定入口为main.go）
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o order-service ./main.go

# 最终阶段
FROM alpine:3.21.3

# 安装运行时依赖
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /app

# 从构建阶段复制二进制文件
COPY --from=builder /app/order-service .

# 暴露服务端口
EXPOSE 8080

# 设置健康检查
HEALTHCHECK --interval=30s --timeout=3s \
  CMD wget --spider http://locacdlhost:8080/health || exit 1

# 使用非root用户运行
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
USER appuser

# 设置默认环境变量（可在运行时覆盖）
ENV DB_HOST=host.docker.internal \
    DB_PORT=3306 \
    DB_USER=root \
    DB_NAME=ecommerce \
    RABBITMQ_URL=amqp://admin:rabbitmq@IP:5672/ \
    ORDER_EXCHANGE=orders_exchange \
    ORDER_QUEUE=orders_queue \
    DEAD_LETTER_QUEUE=dead_letter_queue \
    DELAY_EXCHANGE=delay_exchange

# 启动服务
CMD ["./order-service"]
