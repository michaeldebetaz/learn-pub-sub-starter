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
	slog.Info("Starting Peril client...")

	conn, err := amqp.Dial(URL)
	if err != nil {
		err := fmt.Errorf("Error: failed to connect to AMQP: %w", err)
		log.Fatal(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		err := fmt.Errorf("Error: failed to open channel: %w", err)
		log.Fatal(err)
	}

	username, err := gamelogic.ClientWelcome()
	if err != nil {
		err := fmt.Errorf("Error: failed to welcome client: %w", err)
		log.Fatal(err)
	}

	gs := gamelogic.NewGameState(username)

	if err := subscribeToPerilDirect(conn, gs, username); err != nil {
		err := fmt.Errorf("Error: failed to subscribe to Peril Direct: %w", err)
		log.Fatal(err)
	}

	if err := subscribeToArmyMoves(conn, gs, username); err != nil {
		err := fmt.Errorf("Error: failed to subscribe to Army Moves: %w", err)
		log.Fatal(err)
	}

	for {
		words := gamelogic.GetInput()
		if len(words) == 0 {
			continue
		}

		command := words[0]

		switch command {
		case "spawn":
			if err := gs.CommandSpawn(words); err != nil {
				err := fmt.Errorf("Error: failed to execute spawn command: %w", err)
				log.Print(err)
			}

		case "move":
			move, err := gs.CommandMove(words)
			if err != nil {
				err := fmt.Errorf("Error: failed to execute move command: %w", err)
				log.Print(err)
				continue
			}

			exchange := routing.ExchangePerilTopic
			key := routing.ArmyMovesPrefix + "." + username
			if err := pubsub.PublishJSON(ch, exchange, key, move); err != nil {
				err := fmt.Errorf("Error: failed to publish move command: %w", err)
				log.Print(err)
			}
			slog.Info("Move published", "move", move)

		case "status":
			gs.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()

		case "spam":
			slog.Info("Spamming not allowed yet!")

		case "quit":
			slog.Info("Quitting game...")
			return

		default:
			slog.Error("Unrecognized command")
			gamelogic.PrintClientHelp()
		}
	}
}

func subscribeToPerilDirect(conn *amqp.Connection, gs *gamelogic.GameState, username string) error {
	exchange := routing.ExchangePerilDirect
	queueName := routing.PauseKey + "." + username
	key := routing.PauseKey
	queueType := pubsub.QueueTypeTransient
	if err := pubsub.SubscribeJSON(conn, exchange, queueName, key, queueType, handlerPause(gs)); err != nil {
		err := fmt.Errorf("Error: failed to declare and bind queue: %w", err)
		return err
	}
	return nil
}

func subscribeToArmyMoves(conn *amqp.Connection, gs *gamelogic.GameState, username string) error {
	exchange := routing.ExchangePerilTopic
	queueName := routing.ArmyMovesPrefix + "." + username
	key := routing.ArmyMovesPrefix + ".*"
	queueType := pubsub.QueueTypeTransient
	if err := pubsub.SubscribeJSON(conn, exchange, queueName, key, queueType, handlerArmyMove(gs)); err != nil {
		err := fmt.Errorf("Error: failed to declare and bind queue: %w", err)
		return err
	}
	return nil
}

func handlerArmyMove(gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	defer fmt.Print("> ")
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")

		outcome := gs.HandleMove(move)
		if outcome == gamelogic.MoveOutcomeSafe || outcome == gamelogic.MoveOutcomeMakeWar {
			return pubsub.Ack
		} else {
			return pubsub.NackDiscard
		}
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}
