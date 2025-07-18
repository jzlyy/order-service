package consumers

import (
	amqp "github.com/rabbitmq/amqp091-go"
	"log"
	"order-service/config"
	"order-service/database"
	"strconv"
	"strings"
)

func StartOrderConsumer(ch *amqp.Channel, cfg *config.Config) {
	// 消费主订单队列
	msgs, err := ch.Consume(
		cfg.OrderQueue,
		"order-service", // consumers tag
		false,           // auto-ack
		false,           // exclusive
		false,           // no-local
		false,           // no-wait
		nil,
	)
	if err != nil {
		log.Fatalf("Failed to register consumers: %v", err)
	}

	go func() {
		for msg := range msgs {
			processOrderMessage(msg)
		}
	}()

	// 消费死信队列
	dlqMsgs, err := ch.Consume(
		cfg.DeadLetterQueue,
		"order-service-dlq", // consumers tag
		false,               // auto-ack
		false,               // exclusive
		false,               // no-local
		false,               // no-wait
		nil,
	)
	if err != nil {
		log.Printf("Failed to register DLQ consumers: %v", err)
	}

	go func() {
		for msg := range dlqMsgs {
			processDeadLetterMessage(msg)
		}
	}()
}

func processOrderMessage(msg amqp.Delivery) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Recovered from panic in message processing: %v", r)
		}
	}()

	parts := strings.Split(string(msg.Body), "|")
	if len(parts) < 2 {
		log.Printf("Invalid message format: %s", msg.Body)
		err := msg.Nack(false, false)
		if err != nil {
			return
		} // 拒绝消息，不重新入队
		return
	}

	orderID, err := strconv.Atoi(parts[0])
	if err != nil {
		log.Printf("Invalid order ID: %s", parts[0])
		err := msg.Nack(false, false)
		if err != nil {
			return
		}
		return
	}

	eventType := parts[1]
	log.Printf("Processing order event: ID=%d, Type=%s", orderID, eventType)

	// 根据事件类型处理
	switch eventType {
	case "created":
		handleOrderCreated(orderID)
	case "status_updated":
		handleStatusUpdated(orderID)
	case "payment_check":
		handlePaymentCheck(orderID)
	default:
		log.Printf("Unknown event type: %s", eventType)
	}

	// 处理成功后确认消息
	err = msg.Ack(false)
	if err != nil {
		return
	}
}

func processDeadLetterMessage(msg amqp.Delivery) {
	log.Printf("Received dead letter: %s", msg.Body)
	// 实际处理：记录到数据库、通知管理员等
	err := msg.Ack(false)
	if err != nil {
		return
	}
}

func handleOrderCreated(orderID int) {
	// 实际业务逻辑：通知其他服务、更新缓存等
	log.Printf("Handling order created: %d", orderID)
}

func handleStatusUpdated(orderID int) {
	// 获取订单最新状态
	var status string
	err := database.DB.QueryRow("SELECT status FROM orders WHERE id = ?", orderID).Scan(&status)
	if err != nil {
		log.Printf("Failed to get order status: %v", err)
		return
	}

	// 根据状态处理
	switch status {
	case "shipped":
		// 发送发货通知
	case "cancelled":
		// 处理取消逻辑
	}
	log.Printf("Handling status update for order %d: %s", orderID, status)
}

func handlePaymentCheck(orderID int) {
	// 检查订单支付状态
	var status string
	err := database.DB.QueryRow("SELECT status FROM orders WHERE id = ?", orderID).Scan(&status)
	if err != nil {
		log.Printf("Failed to get order status: %v", err)
		return
	}

	// 如果订单仍未支付，自动取消
	if status == "pending" {
		_, err := database.DB.Exec(
			"UPDATE orders SET status = 'cancelled', updated_at = NOW() WHERE id = ?",
			orderID,
		)
		if err != nil {
			log.Printf("Failed to auto-cancel order %d: %v", orderID, err)
		} else {
			log.Printf("Auto-cancelled order %d due to non-payment", orderID)
		}
	}
}
