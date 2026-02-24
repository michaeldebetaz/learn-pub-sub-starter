package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/bootdotdev/learn-pub-sub-starter/internal/gamelogic"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/pubsub"
	"github.com/bootdotdev/learn-pub-sub-starter/internal/routing"
	amqp "github.com/rabbitmq/amqp091-go"
)

const URL = "amqp://guest:guest@localhost:5672/"

func main() {
	slog.Info("Starting Peril server...")

	conn, err := amqp.Dial(URL)
	if err != nil {
		err := fmt.Errorf("Error: failed to connect to AMQP: %w", err)
		log.Fatal(err)
	}
	defer conn.Close()

	slog.Info("Server connected to AMQP.")

	exchange := routing.ExchangePerilTopic
	queueName := routing.GameLogSlug
	key := routing.GameLogSlug + ".*"
	queueType := pubsub.QueueTypeDurable
	ch, queue, err := pubsub.DeclareAndBind(conn, exchange, queueName, key, queueType)
	if err != nil {
		err := fmt.Errorf("Error: failed to declare and bind queue: %w", err)
		log.Fatal(err)
	}

	slog.Info("Server declared and bound queue.", "queueName", queue.Name)

	gamelogic.PrintServerHelp()

	for {
		inputs := gamelogic.GetInput()
		if len(inputs) == 0 {
			continue
		}

		command := inputs[0]
		switch command {
		case "pause":
			slog.Info("Error: pausing game...")

			playingState := routing.PlayingState{IsPaused: true}
			if err := publishPlayingState(ch, playingState); err != nil {
				err := fmt.Errorf("Error: failed to pause game: %w", err)
				log.Print(err)
			}

		case "resume":
			slog.Info("Error: resuming game...")

			playingState := routing.PlayingState{IsPaused: false}
			if err := publishPlayingState(ch, playingState); err != nil {
				err := fmt.Errorf("Error: failed to unpause game: %w", err)
				log.Print(err)
			}

		case "quit":
			slog.Info("Error: quitting game...")
			return

		default:
			slog.Info("Error: unrecognized command")
			gamelogic.PrintServerHelp()
			continue
		}
	}
}

func publishPlayingState(ch *amqp.Channel, playingState routing.PlayingState) error {
	exchange := routing.ExchangePerilDirect
	key := routing.PauseKey
	if err := pubsub.PublishJSON(ch, exchange, key, playingState); err != nil {
		err := fmt.Errorf("failed to publish JSON: %w", err)
		return err
	}

	return nil
}
