package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const URL = "amqp://guest:guest@localhost:5672/"

func main() {
	slog.Info("Starting Peril server...")

	conn, err := amqp.Dial(URL)
	if err != nil {
		err := fmt.Errorf("Failed to connect to AMQP: %w", err)
		log.Fatal(err)
	}
	defer conn.Close()

	slog.Info("Successfully connected to AMQP")

	channel, err := conn.Channel()
	if err != nil {
		err := fmt.Errorf("Failed to open a channel: %w", err)
		log.Fatal(err)
	}
	defer channel.Close()

	if err := pubsub.PublishJSON(channel, routing.ExchangePerilDirect, routing.PauseKey, routing.PlayingState{IsPaused: true}); err != nil {
		err := fmt.Errorf("Failed to publish initial message: %w", err)
		log.Fatal(err)
	}

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
}
