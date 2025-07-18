package controllers

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"order-service/database"
	"order-service/models"
	"order-service/rabbitmq"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"order-service/middlewares"
)

var rabbitMQ *rabbitmq.RabbitMQ

func SetRabbitMQ(rmq *rabbitmq.RabbitMQ) {
	rabbitMQ = rmq
}

func CreateOrder(c *gin.Context) {
	defer func() {
		status := c.Writer.Status() >= 200 && c.Writer.Status() < 300
		middlewares.RecordOrderOperation("create", status)
	}()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	var order models.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// 验证订单项
	if len(order.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Order must contain at least one item"})
		return
	}

	// 设置用户ID
	order.UserID = userID.(int)
	order.Status = "pending"
	order.CreatedAt = time.Now()
	order.UpdatedAt = time.Now()

	// 计算总价
	for _, item := range order.Items {
		order.Total += item.Price * float64(item.Quantity)
	}

	// 开始事务
	tx, err := database.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not start transaction"})
		return
	}

	// 插入订单
	orderResult, err := tx.Exec(
		"INSERT INTO orders (user_id, total, status, created_at, updated_at) VALUES (?, ?, ?, ?, ?)",
		order.UserID, order.Total, order.Status, order.CreatedAt, order.UpdatedAt,
	)
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create order"})
		return
	}

	orderID, err := orderResult.LastInsertId()
	if err != nil {
		err := tx.Rollback()
		if err != nil {
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order ID"})
		return
	}

	// 插入订单项
	for _, item := range order.Items {
		_, err = tx.Exec(
			"INSERT INTO order_items (order_id, product_id, product_name, quantity, price) VALUES (?, ?, ?, ?, ?)",
			orderID, item.ProductID, item.ProductName, item.Quantity, item.Price,
		)
		if err != nil {
			err := tx.Rollback()
			if err != nil {
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add order item"})
			return
		}
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Transaction commit failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"order_id": orderID})

	// 事务提交成功后发送事件
	if rabbitMQ != nil {
		// 高优先级事件（VIP用户）
		priority := 5           // 默认优先级
		if order.Total > 1000 { // 大额订单高优先级
			priority = 9
		}

		if err := rabbitMQ.PublishOrderEvent(int(orderID), priority, "created"); err != nil {
			log.Printf("Failed to publish order created event: %v", err)
		}

		// 设置延迟事件（15分钟后检查支付状态）
		if err := rabbitMQ.PublishDelayedEvent(int(orderID), 15*time.Minute, "payment_check"); err != nil {
			log.Printf("Failed to publish delayed payment check event: %v", err)
		}
	}

	c.JSON(http.StatusCreated, gin.H{"order_id": orderID})
}

func GetUserOrders(c *gin.Context) {
	defer func() {
		status := c.Writer.Status() >= 200 && c.Writer.Status() < 300
		middlewares.RecordOrderOperation("list", status)
	}()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	rows, err := database.DB.Query(`
		SELECT o.id, o.total, o.status, o.created_at, 
		       oi.id, oi.product_id, oi.product_name, oi.quantity, oi.price
		FROM orders o
		JOIN order_items oi ON o.id = oi.order_id
		WHERE o.user_id = ?
		ORDER BY o.created_at DESC, oi.id ASC
	`, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	ordersMap := make(map[int]*models.OrderResponse)
	for rows.Next() {
		var (
			orderID     int
			total       float64
			status      string
			createdAt   time.Time
			itemID      int
			productID   int
			productName string
			quantity    int
			price       float64
		)

		if err := rows.Scan(&orderID, &total, &status, &createdAt,
			&itemID, &productID, &productName, &quantity, &price); err != nil {
			log.Printf("Error scanning row: %v", err)
			continue
		}

		if _, exists := ordersMap[orderID]; !exists {
			ordersMap[orderID] = &models.OrderResponse{
				ID:        orderID,
				UserID:    userID.(int),
				Total:     total,
				Status:    status,
				CreatedAt: createdAt,
				Items:     []models.OrderItemDetail{},
			}
		}

		ordersMap[orderID].Items = append(ordersMap[orderID].Items, models.OrderItemDetail{
			ProductID:   productID,
			ProductName: productName,
			Quantity:    quantity,
			Price:       price,
			Subtotal:    price * float64(quantity),
		})
	}

	orders := make([]models.OrderResponse, 0, len(ordersMap))
	for _, order := range ordersMap {
		orders = append(orders, *order)
	}

	c.JSON(http.StatusOK, orders)
}

func GetOrderDetails(c *gin.Context) {
	defer func() {
		status := c.Writer.Status() >= 200 && c.Writer.Status() < 300
		middlewares.RecordOrderOperation("details", status)
	}()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	// 查询订单基本信息
	var order models.OrderResponse
	err = database.DB.QueryRow(`
		SELECT id, user_id, total, status, created_at
		FROM orders
		WHERE id = ? AND user_id = ?
	`, orderID, userID).Scan(
		&order.ID, &order.UserID, &order.Total, &order.Status, &order.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Order not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// 查询订单项
	rows, err := database.DB.Query(`
		SELECT product_id, product_name, quantity, price
		FROM order_items
		WHERE order_id = ?
	`, orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get order items"})
		return
	}
	defer func(rows *sql.Rows) {
		err := rows.Close()
		if err != nil {

		}
	}(rows)

	for rows.Next() {
		var item models.OrderItemDetail
		if err := rows.Scan(&item.ProductID, &item.ProductName, &item.Quantity, &item.Price); err != nil {
			log.Printf("Error scanning order item: %v", err)
			continue
		}
		item.Subtotal = item.Price * float64(item.Quantity)
		order.Items = append(order.Items, item)
	}

	c.JSON(http.StatusOK, order)
}

func UpdateOrderStatus(c *gin.Context) {
	defer func() {
		status := c.Writer.Status() >= 200 && c.Writer.Status() < 300
		middlewares.RecordOrderOperation("update_status", status)
	}()
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	orderID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid order ID"})
		return
	}

	var request struct {
		Status string `json:"status" binding:"required,oneof=pending processing shipped delivered cancelled"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	result, err := database.DB.Exec(`
		UPDATE orders 
		SET status = ?, updated_at = NOW()
		WHERE id = ? AND user_id = ?
	`, request.Status, orderID, userID)

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "Order not found or not authorized"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated", "order_id": orderID})

	if rabbitMQ != nil {
		priority := 5                      // 默认优先级
		if request.Status == "cancelled" { // 取消订单高优先级
			priority = 8
		}

		if err := rabbitMQ.PublishOrderEvent(orderID, priority, "status_updated"); err != nil {
			log.Printf("Failed to publish order updated event: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{"message": "Order status updated", "order_id": orderID})
}

// HandleDeadLetter 死信队列处理函数
func HandleDeadLetter(c *gin.Context) {
	defer func() {
		status := c.Writer.Status() >= 200 && c.Writer.Status() < 300
		middlewares.RecordOrderOperation("dead_letter", status)
	}()

	var deadLetter struct {
		OrderID int    `json:"order_id"`
		Reason  string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&deadLetter); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Handling dead letter for order %d: %s", deadLetter.OrderID, deadLetter.Reason)

	// 实际处理逻辑：记录、通知管理员等
	c.JSON(http.StatusOK, gin.H{"message": "Dead letter processed"})
}
