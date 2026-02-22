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
	slog.Info("CLIENT: starting Peril client...")

	conn, err := amqp.Dial(URL)
	if err != nil {
		err := fmt.Errorf("CLIENT: failed to connect to AMQP: %w", err)
		log.Fatal(err)
	}
	defer conn.Close()

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		err := fmt.Errorf("CLIENT: failed to welcome client: %w", err)
		log.Fatal(err)
	}

	exchange := routing.ExchangePerilDirect
	queueName := routing.PauseKey + "." + username
	key := routing.PauseKey
	queueType := pubsub.QueueTypeTransient
	if _, _, err := pubsub.DeclareAndBind(conn, exchange, queueName, key, queueType); err != nil {
		err := fmt.Errorf("CLIENT: failed to declare and bind queue: %w", err)
		log.Fatal(err)
	}

	gamestate := gamelogic.NewGameState(username)

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		command := words[0]

		switch command {
		case "spawn":
			if err := gamestate.CommandSpawn(words); err != nil {
				err := fmt.Errorf("CLIENT: failed to execute spawn command: %w", err)
				log.Print(err)
			}

		case "move":
			move, err := gamestate.CommandMove(words)
			if err != nil {
				err := fmt.Errorf("CLIENT: failed to execute move command: %w", err)
				log.Print(err)
				continue
			}
			slog.Info("CLIENT: move successful", "move", move)

		case "status":
			gamestate.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()

		case "spam":
			slog.Info("CLIENT: spamming not allowed yet!")

		case "quit":
			slog.Info("CLIENT: quitting game...")
			return

		default:
			slog.Info("CLIENT: unrecognized command")
			gamelogic.PrintClientHelp()
		}
	}
}
