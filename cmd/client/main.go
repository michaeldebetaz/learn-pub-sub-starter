package main

import (
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"time"

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

	if err := subscribeToWars(conn, gs); err != nil {
		err := fmt.Errorf("Error: failed to subscribe to Wars: %w", err)
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
				slog.Error("Error: failed to execute spawn command", "error", err)
				continue
			}

		case "move":
			move, err := gs.CommandMove(words)
			if err != nil {
				slog.Error("Error: failed to execute move command", "error", err)
				continue
			}

			exchange := routing.ExchangePerilTopic
			key := routing.ArmyMovesPrefix + "." + username
			if err := pubsub.PublishJSON(ch, exchange, key, move); err != nil {
				slog.Error("Failed to publish move command", "error", err)
				continue
			}

			slog.Info("Move published", "move", move)

		case "status":
			gs.CommandStatus()

		case "help":
			gamelogic.PrintClientHelp()

		case "spam":
			if len(words) < 2 {
				slog.Error("Usage: spam <number of moves>")
				continue
			}

			n, err := strconv.Atoi(words[1])
			if err != nil {
				slog.Error("Invalid number for spam command", "input", words[1], "error", err)
				continue
			}

			for range n {
				log := gamelogic.GetMaliciousLog()
				exchange := routing.ExchangePerilTopic
				key := routing.GameLogSlug + "." + username
				pubsub.PublishGob(ch, exchange, key, routing.GameLog{
					CurrentTime: time.Now(),
					Message:     log,
					Username:    username,
				})
			}

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
		err := fmt.Errorf("failed to declare and bind queue: %w", err)
		return err
	}
	return nil
}

func subscribeToArmyMoves(conn *amqp.Connection, gs *gamelogic.GameState, username string) error {
	exchange := routing.ExchangePerilTopic
	queueName := routing.ArmyMovesPrefix + "." + username
	key := routing.ArmyMovesPrefix + ".*"
	queueType := pubsub.QueueTypeTransient
	if err := pubsub.SubscribeJSON(conn, exchange, queueName, key, queueType, handlerArmyMove(conn, gs)); err != nil {
		err := fmt.Errorf("failed to declare and bind queue: %w", err)
		return err
	}
	return nil
}

func subscribeToWars(conn *amqp.Connection, gs *gamelogic.GameState) error {
	exchange := routing.ExchangePerilTopic
	queueName := "war"
	key := routing.WarRecognitionsPrefix + ".*"
	queueType := pubsub.QueueTypeDurable
	if err := pubsub.SubscribeJSON(conn, exchange, queueName, key, queueType, handlerWar(conn, gs)); err != nil {
		err := fmt.Errorf("failed to declare and bind queue: %w", err)
		return err
	}
	return nil
}

func handlerArmyMove(conn *amqp.Connection, gs *gamelogic.GameState) func(gamelogic.ArmyMove) pubsub.AckType {
	return func(move gamelogic.ArmyMove) pubsub.AckType {
		defer fmt.Print("> ")

		outcome := gs.HandleMove(move)
		switch outcome {
		case gamelogic.MoveOutcomeSafe:
			return pubsub.Ack

		case gamelogic.MoveOutcomeMakeWar:
			ch, err := conn.Channel()
			if err != nil {
				slog.Error("Failed to open channel", "error", err)
				return pubsub.NackDiscard
			}
			exchange := routing.ExchangePerilTopic
			key := routing.WarRecognitionsPrefix + "." + gs.GetUsername()
			val := gamelogic.RecognitionOfWar{
				Attacker: move.Player,
				Defender: gs.GetPlayerSnap(),
			}
			if err := pubsub.PublishJSON(ch, exchange, key, val); err != nil {
				slog.Error("Failed to publish war recognition", "error", err)
				return pubsub.NackRequeue
			}

			return pubsub.Ack
		}

		return pubsub.NackDiscard
	}
}

func handlerPause(gs *gamelogic.GameState) func(routing.PlayingState) pubsub.AckType {
	return func(ps routing.PlayingState) pubsub.AckType {
		defer fmt.Print("> ")
		gs.HandlePause(ps)
		return pubsub.Ack
	}
}

func handlerWar(conn *amqp.Connection, gs *gamelogic.GameState) func(gamelogic.RecognitionOfWar) pubsub.AckType {
	return func(rw gamelogic.RecognitionOfWar) pubsub.AckType {
		defer fmt.Print("> ")

		ch, err := conn.Channel()
		if err != nil {
			slog.Error("Failed to open channel", "error", err)
			return pubsub.NackDiscard
		}

		outcome, winner, loser := gs.HandleWar(rw)
		switch outcome {
		case gamelogic.WarOutcomeNotInvolved:
			return pubsub.NackRequeue
		case gamelogic.WarOutcomeNoUnits:
			return pubsub.NackDiscard
		case gamelogic.WarOutcomeOpponentWon:
			message := fmt.Sprintf("%s won a war against %s", winner, loser)
			if err := pubsub.PublishGamelog(ch, gs.GetUsername(), message); err != nil {
				slog.Error("Failed to publish gamelog", "error", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeYouWon:
			message := fmt.Sprintf("%s won a war against %s", winner, loser)
			if err := pubsub.PublishGamelog(ch, gs.GetUsername(), message); err != nil {
				slog.Error("Failed to publish gamelog", "error", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		case gamelogic.WarOutcomeDraw:
			message := fmt.Sprintf("A war between %s and %s resulted in a draw", winner, loser)
			if err := pubsub.PublishGamelog(ch, gs.GetUsername(), message); err != nil {
				slog.Error("Failed to publish gamelog", "error", err)
				return pubsub.NackRequeue
			}
			return pubsub.Ack
		default:
			slog.Error("Invalid war outcome", "outcome", outcome)
			return pubsub.NackDiscard
		}
	}
}
