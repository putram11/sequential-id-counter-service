package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/putram11/sequential-id-counter-service/internal/config"
	"github.com/putram11/sequential-id-counter-service/internal/models"
	"github.com/streadway/amqp"
)

// RabbitMQRepository handles RabbitMQ operations
type RabbitMQRepository struct {
	conn         *amqp.Connection
	channel      *amqp.Channel
	exchangeName string
	queueName    string
}

// NewRabbitMQRepository creates a new RabbitMQ repository
func NewRabbitMQRepository(cfg config.RabbitMQConfig) (*RabbitMQRepository, error) {
	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	
	channel, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}
	
	// Declare exchange
	err = channel.ExchangeDeclare(
		cfg.Exchange, // name
		"direct",     // type
		true,         // durable
		false,        // auto-deleted
		false,        // internal
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}
	
	// Declare queue
	_, err = channel.QueueDeclare(
		cfg.Queue, // name
		true,      // durable
		false,     // delete when unused
		false,     // exclusive
		false,     // no-wait
		amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": cfg.Queue + "_dlq",
			"x-message-ttl":             86400000, // 24 hours in milliseconds
		}, // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare queue: %w", err)
	}
	
	// Declare dead letter queue
	_, err = channel.QueueDeclare(
		cfg.Queue+"_dlq", // name
		true,             // durable
		false,            // delete when unused
		false,            // exclusive
		false,            // no-wait
		nil,              // arguments
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to declare dead letter queue: %w", err)
	}
	
	// Bind queue to exchange
	err = channel.QueueBind(
		cfg.Queue,    // queue name
		"seq.log",    // routing key
		cfg.Exchange, // exchange
		false,
		nil,
	)
	if err != nil {
		channel.Close()
		conn.Close()
		return nil, fmt.Errorf("failed to bind queue: %w", err)
	}
	
	return &RabbitMQRepository{
		conn:         conn,
		channel:      channel,
		exchangeName: cfg.Exchange,
		queueName:    cfg.Queue,
	}, nil
}

// PublishEvent publishes an event to the queue
func (r *RabbitMQRepository) PublishEvent(ctx context.Context, event *models.Event) error {
	// Set published timestamp
	event.PublishedAt = time.Now()
	
	// Marshal event to JSON
	body, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal event: %w", err)
	}
	
	// Publish message
	err = r.channel.Publish(
		r.exchangeName, // exchange
		"seq.log",      // routing key
		false,          // mandatory
		false,          // immediate
		amqp.Publishing{
			DeliveryMode:  amqp.Persistent, // persistent message
			ContentType:   "application/json",
			Body:          body,
			MessageId:     event.MessageID,
			Timestamp:     event.PublishedAt,
			CorrelationId: event.CorrelationID,
			Headers: amqp.Table{
				"prefix":      event.Prefix,
				"counter":     event.Counter,
				"retry_count": event.RetryCount,
			},
		},
	)
	
	if err != nil {
		return fmt.Errorf("failed to publish event: %w", err)
	}
	
	return nil
}

// ConsumeEvents starts consuming events from the queue
func (r *RabbitMQRepository) ConsumeEvents(ctx context.Context, handler func(*models.Event) error) error {
	// Set QoS to limit unacknowledged messages
	err := r.channel.Qos(
		10,    // prefetch count
		0,     // prefetch size
		false, // global
	)
	if err != nil {
		return fmt.Errorf("failed to set QoS: %w", err)
	}
	
	// Start consuming
	msgs, err := r.channel.Consume(
		r.queueName, // queue
		"",          // consumer
		false,       // auto-ack
		false,       // exclusive
		false,       // no-local
		false,       // no-wait
		nil,         // args
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}
	
	// Process messages
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-msgs:
			if !ok {
				return fmt.Errorf("channel closed")
			}
			
			// Parse event
			var event models.Event
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				msg.Nack(false, false) // Send to DLQ
				continue
			}
			
			// Handle event
			if err := handler(&event); err != nil {
				// Increment retry count
				event.RetryCount++
				
				// If retry count exceeds limit, reject to DLQ
				if event.RetryCount >= 3 {
					msg.Nack(false, false)
				} else {
					// Requeue with delay (simplified - in production use a delay exchange)
					msg.Nack(false, true)
				}
				continue
			}
			
			// Acknowledge successful processing
			msg.Ack(false)
		}
	}
}

// GetQueueInfo returns information about the queue
func (r *RabbitMQRepository) GetQueueInfo(ctx context.Context) (map[string]interface{}, error) {
	queue, err := r.channel.QueueInspect(r.queueName)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect queue: %w", err)
	}
	
	info := map[string]interface{}{
		"name":      queue.Name,
		"messages":  queue.Messages,
		"consumers": queue.Consumers,
	}
	
	return info, nil
}

// Ping checks RabbitMQ connectivity
func (r *RabbitMQRepository) Ping(ctx context.Context) error {
	if r.conn.IsClosed() {
		return fmt.Errorf("connection is closed")
	}
	
	// Try to declare a temporary queue to test connectivity
	tempQueue := fmt.Sprintf("health_check_%d", time.Now().UnixNano())
	_, err := r.channel.QueueDeclare(
		tempQueue,
		false, // durable
		true,  // delete when unused
		true,  // exclusive
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	
	// Clean up
	_, err = r.channel.QueueDelete(tempQueue, false, false, false)
	if err != nil {
		// Log but don't fail health check
		fmt.Printf("Warning: failed to clean up health check queue: %v\n", err)
	}
	
	return nil
}

// Close closes the RabbitMQ connection
func (r *RabbitMQRepository) Close() error {
	if r.channel != nil {
		r.channel.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

// GetStats returns connection statistics
func (r *RabbitMQRepository) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"connection_closed": r.conn.IsClosed(),
	}
	
	if !r.conn.IsClosed() {
		stats["local_addr"] = r.conn.LocalAddr().String()
		stats["remote_addr"] = r.conn.RemoteAddr().String()
	}
	
	return stats
}
