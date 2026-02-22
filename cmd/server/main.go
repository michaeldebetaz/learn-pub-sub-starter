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
	slog.Info("SERVER: starting Peril server...")

	conn, err := amqp.Dial(URL)
	if err != nil {
		err := fmt.Errorf("SERVER: failed to connect to AMQP: %w", err)
		log.Fatal(err)
	}
	defer conn.Close()

	slog.Info("SERVER: successfully connected to AMQP")

	exchange := routing.ExchangePerilTopic
	queueName := routing.GameLogSlug
	key := routing.GameLogSlug + ".*"
	queueType := pubsub.QueueTypeDurable
	if _, _, err := pubsub.DeclareAndBind(conn, exchange, queueName, key, queueType); err != nil {
		err := fmt.Errorf("SERVER: failed to declare and bind queue: %w", err)
		log.Fatal(err)
	}

	gamelogic.PrintServerHelp()

	ch, err := conn.Channel()
	if err != nil {
		err := fmt.Errorf("SERVER: failed to open a channel: %w", err)
		log.Fatal(err)
	}
	defer ch.Close()

	for {
		inputs := gamelogic.GetInput()
		if len(inputs) == 0 {
			continue
		}

		command := inputs[0]
		switch command {
		case "pause":
			slog.Info("SERVER: pausing game...")

			playingState := routing.PlayingState{IsPaused: true}
			if err := publishPlayingState(ch, playingState); err != nil {
				err := fmt.Errorf("SERVER: failed to pause game: %w", err)
				log.Print(err)
			}

		case "resume":
			slog.Info("SERVER: resuming game...")

			playingState := routing.PlayingState{IsPaused: false}
			if err := publishPlayingState(ch, playingState); err != nil {
				err := fmt.Errorf("SERVER: failed to unpause game: %w", err)
				log.Print(err)
			}

		case "quit":
			slog.Info("SERVER: quitting game...")
			return

		default:
			slog.Info("SERVER: unrecognized command")
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
