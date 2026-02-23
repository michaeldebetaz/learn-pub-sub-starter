package pubsub

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type SimpleQueueType int

const (
	QueueTypeDurable SimpleQueueType = iota
	QueueTypeTransient
)

var typeName = map[SimpleQueueType]string{
	QueueTypeDurable:   "durable",
	QueueTypeTransient: "transient",
}

func (s SimpleQueueType) String() string {
	return typeName[s]
}

func PublishJSON[T any](ch *amqp.Channel, exchange string, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		err := fmt.Errorf("failed to marshal JSON: %w", err)
		return err
	}

	if err := ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	}); err != nil {
		err := fmt.Errorf("failed to publish message: %w", err)
		return err
	}

	return nil
}

func DeclareAndBind(conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType) (*amqp.Channel, amqp.Queue, error) {
	ch, err := conn.Channel()
	if err != nil {
		err := fmt.Errorf("failed to create channel: %w", err)
		return nil, amqp.Queue{}, err
	}

	durable := queueType == QueueTypeDurable
	autoDelete := queueType == QueueTypeTransient
	exclusive := queueType == QueueTypeTransient
	noWait := false
	queue, err := ch.QueueDeclare(queueName, durable, autoDelete, exclusive, noWait, nil)
	if err != nil {
		err := fmt.Errorf("failed to declare queue: %w", err)
		return nil, amqp.Queue{}, err
	}

	if err := ch.QueueBind(queueName, key, exchange, noWait, nil); err != nil {
		err := fmt.Errorf("failed to bind queue: %w", err)
		return nil, amqp.Queue{}, err
	}

	return ch, queue, nil
}

func SubscribeJSON[T any](conn *amqp.Connection, exchange, queueName, key string, queueType SimpleQueueType, handler func(T)) error {
	ch, queue, err := DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		err := fmt.Errorf("failed to declare and bind: %w", err)
		return err
	}

	deliveriesCh, err := ch.Consume(queue.Name, "", false, false, false, false, nil)
	if err != nil {
		err := fmt.Errorf("failed to consume messages: %w", err)
		return err
	}

	go func() {
		for delivery := range deliveriesCh {
			var v T
			if err := json.Unmarshal(delivery.Body, &v); err != nil {
				slog.Error("failed to unmarshal delivery body", "error", err)
				delivery.Nack(false, false)
				continue
			}

			handler(v)
		}
	}()

	return nil
}
