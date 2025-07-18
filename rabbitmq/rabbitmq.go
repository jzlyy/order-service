package rabbitmq

import (
	"log"
	"order-service/config"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	Conn    *amqp.Connection
	Channel *amqp.Channel
	Cfg     *config.Config
}

func NewRabbitMQ(cfg *config.Config) (*RabbitMQ, error) {
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		err := conn.Close()
		if err != nil {
			return nil, err
		}
		return nil, err
	}

	return &RabbitMQ{
		Conn:    conn,
		Channel: ch,
		Cfg:     cfg,
	}, nil
}

func (r *RabbitMQ) SetupQueues() error {
	// 声明死信交换机和队列
	if err := r.Channel.ExchangeDeclare(
		r.Cfg.DeadLetterQueue+"_exchange",
		"direct",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return err
	}

	_, err := r.Channel.QueueDeclare(
		r.Cfg.DeadLetterQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-queue-type": "classic", // 明确指定队列类型
		},
	)
	if err != nil {
		return err
	}

	if err := r.Channel.QueueBind(
		r.Cfg.DeadLetterQueue,
		"",
		r.Cfg.DeadLetterQueue+"_exchange",
		false,
		nil,
	); err != nil {
		return err
	}

	// 声明延迟交换机（需要RabbitMQ安装延迟插件）
	if err := r.Channel.ExchangeDeclare(
		r.Cfg.DelayExchange,
		"x-delayed-message",
		true,  // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		amqp.Table{"x-delayed-type": "direct"},
	); err != nil {
		log.Printf("Warning: Delayed exchange not supported: %v", err)
	}

	// 声明主订单队列（带优先级和死信）
	_, err = r.Channel.QueueDeclare(
		r.Cfg.OrderQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		amqp.Table{
			"x-max-priority":            r.Cfg.MaxPriority, // 设置最大优先级
			"x-dead-letter-exchange":    r.Cfg.DeadLetterQueue + "_exchange",
			"x-dead-letter-routing-key": r.Cfg.DeadLetterQueue,
		},
	)
	if err != nil {
		return err
	}

	// 绑定主队列到订单交换机
	if err := r.Channel.QueueBind(
		r.Cfg.OrderQueue,
		"",
		r.Cfg.OrderExchange,
		false,
		nil,
	); err != nil {
		return err
	}

	return nil
}

func (r *RabbitMQ) PublishOrderEvent(orderID int, priority int, eventType string) error {
	body := []byte(string(rune(orderID)) + "|" + eventType)

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "text/plain",
		Body:         body,
		Priority:     uint8(priority),
	}

	return r.Channel.Publish(
		r.Cfg.OrderExchange,
		"",
		false, // mandatory
		false, // immediate
		msg,
	)
}

func (r *RabbitMQ) PublishDelayedEvent(orderID int, delay time.Duration, eventType string) error {
	body := []byte(string(rune(orderID)) + "|" + eventType)

	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "text/plain",
		Body:         body,
		Headers: amqp.Table{
			"x-delay": delay.Milliseconds(), // 延迟时间（毫秒）
		},
	}

	return r.Channel.Publish(
		r.Cfg.DelayExchange,
		"",
		false, // mandatory
		false, // immediate
		msg,
	)
}

func (r *RabbitMQ) Close() {
	if r.Channel != nil {
		err := r.Channel.Close()
		if err != nil {
			return
		}
	}
	if r.Conn != nil {
		err := r.Conn.Close()
		if err != nil {
			return
		}
	}
}
