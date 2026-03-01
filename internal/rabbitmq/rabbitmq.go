package rabbitmq

import (
	"fmt"
	"log"
	"os"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func Connect() *amqp.Connection {
	url := os.Getenv("RABBITMQ_URL")
	if url == "" {
		url = "amqp://guest:guest@localhost:5672/"
	}

	var conn *amqp.Connection
	var err error

	for i := range 10 {
		conn, err = amqp.Dial(url)
		if err == nil {
			return conn
		}
		log.Printf("rabbitmq connection attempt %d/10 failed: %v", i+1, err)
		time.Sleep(2 * time.Second)
	}

	log.Fatalf("failed to connect to rabbitmq after 10 attempts: %v", err)
	return nil
}

func DeclareQueue(ch *amqp.Channel, name string) (amqp.Queue, error) {
	return ch.QueueDeclare(
		name,
		true,
		false,
		false,
		false,
		nil,
	)
}

func Publish(ch *amqp.Channel, queue string, body []byte) error {
	return ch.Publish(
		"",
		queue,
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
		},
	)
}

func Consume(ch *amqp.Channel, queue string) (<-chan amqp.Delivery, error) {
	return ch.Consume(
		queue,
		fmt.Sprintf("%s-consumer", queue),
		false,
		false,
		false,
		false,
		nil,
	)
}
