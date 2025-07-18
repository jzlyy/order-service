package models

import (
	"time"
)

type Order struct {
	ID        int         `json:"id"`
	UserID    int         `json:"user_id"`
	Total     float64     `json:"total" binding:"required"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
	UpdatedAt time.Time   `json:"updated_at"`
	Items     []OrderItem `json:"items" binding:"required"`
}

type OrderItem struct {
	ProductID   int     `json:"product_id" binding:"required"`
	ProductName string  `json:"product_name" binding:"required"`
	Quantity    int     `json:"quantity" binding:"required"`
	Price       float64 `json:"price" binding:"required"`
}

type OrderResponse struct {
	ID        int               `json:"id"`
	UserID    int               `json:"user_id"`
	Total     float64           `json:"total"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
	Items     []OrderItemDetail `json:"items"`
}

type OrderItemDetail struct {
	ProductID   int     `json:"product_id"`
	ProductName string  `json:"product_name"`
	Quantity    int     `json:"quantity"`
	Price       float64 `json:"price"`
	Subtotal    float64 `json:"subtotal"`
}
type OrderEvent struct {
	OrderID  int       `json:"order_id"`
	UserID   int       `json:"user_id"`
	Type     string    `json:"type"` // created, updated, canceled, completed
	Status   string    `json:"status"`
	Total    float64   `json:"total"`
	Occurred time.Time `json:"occurred"`
}
