package main

import (
	"log"
	"net/http"
	"order-service/config"
	"order-service/consumers"
	"order-service/controllers"
	"order-service/database"
	"order-service/middlewares"
	"order-service/rabbitmq"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	// 初始化数据库
	if err := database.InitDB(); err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer database.CloseDB()

	// 加载配置
	cfg := config.LoadConfig()

	// 初始化RabbitMQ
	rmq, err := rabbitmq.NewRabbitMQ(cfg)
	if err != nil {
		log.Fatalf("RabbitMQ initialization failed: %v", err)
	}
	defer rmq.Close()

	// 设置队列和交换机
	if err := rmq.SetupQueues(); err != nil {
		log.Fatalf("Failed to setup RabbitMQ queues: %v", err)
	}

	// 启动消息消费者
	go consumers.StartOrderConsumer(rmq.Channel, cfg)

	// 设置RabbitMQ实例到控制器
	controllers.SetRabbitMQ(rmq)

	// 创建Gin路由
	r := gin.Default()

	// 应用Prometheus中间件
	r.Use(middlewares.PrometheusMiddleware())

	// 暴露Prometheus指标端点
	r.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// 健康检查端点
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 需要认证的路由组
	authGroup := r.Group("/api")
	authGroup.Use(middlewares.AuthMiddleware())
	{
		authGroup.POST("/orders", controllers.CreateOrder)
		authGroup.GET("/orders", controllers.GetUserOrders)
		authGroup.GET("/orders/:id", controllers.GetOrderDetails)
		authGroup.PUT("/orders/:id/status", controllers.UpdateOrderStatus)
	}

	// 死信队列处理端点
	r.POST("/dead-letter", controllers.HandleDeadLetter)

	// 启动服务器
	port := ":8080"
	log.Printf("Order services starting on port %s", port)
	if err := r.Run(port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
