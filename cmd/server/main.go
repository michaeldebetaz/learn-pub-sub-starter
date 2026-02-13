package main

import (
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"

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

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	<-signalChan
}
