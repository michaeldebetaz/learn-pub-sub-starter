package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

func PublishJSON[T any](ch *amqp.Channel, exchange string, key string, val T) error {
	body, err := json.Marshal(val)
	if err != nil {
		err := fmt.Errorf("failed to marshal JSON: %w", err)
		return err
	}

	ch.PublishWithContext(context.Background(), exchange, key, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})

	return nil
}
