package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/smtp"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/zensos/microservice-project/internal/rabbitmq"
)

type BookingEvent struct {
	BookingID   string   `json:"booking_id"`
	MemberID    string   `json:"member_id"`
	MemberEmail string   `json:"member_email"`
	MemberName  string   `json:"member_name"`
	EventName   string   `json:"event_name"`
	SeatIDs     []string `json:"seat_ids"`
	TotalAmount float64  `json:"total_amount"`
}

func main() {
	conn := rabbitmq.Connect()
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("failed to open channel: %v", err)
	}
	defer ch.Close()

	_, err = rabbitmq.DeclareQueue(ch, "booking.confirmed")
	if err != nil {
		log.Fatalf("failed to declare queue: %v", err)
	}

	msgs, err := rabbitmq.Consume(ch, "booking.confirmed")
	if err != nil {
		log.Fatalf("failed to consume: %v", err)
	}

	log.Println("mailer service started, waiting for messages...")

	go func() {
		for msg := range msgs {
			var event BookingEvent
			if err := json.Unmarshal(msg.Body, &event); err != nil {
				log.Printf("failed to parse message: %v", err)
				msg.Nack(false, false)
				continue
			}

			log.Printf("sending confirmation email for booking %s to %s", event.BookingID, event.MemberEmail)

			if err := sendEmail(event); err != nil {
				log.Printf("failed to send email: %v", err)
				msg.Nack(false, true)
				continue
			}

			log.Printf("email sent for booking %s", event.BookingID)
			msg.Ack(false)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("mailer service shutting down...")
}

func sendEmail(event BookingEvent) error {
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost == "" {
		smtpHost = "smtp.gmail.com"
	}
	smtpPort := os.Getenv("SMTP_PORT")
	if smtpPort == "" {
		smtpPort = "587"
	}
	senderEmail := os.Getenv("SMTP_EMAIL")
	senderPassword := os.Getenv("SMTP_PASSWORD")

	if senderEmail == "" || senderPassword == "" {
		log.Printf("[MOCK] would send email to %s for booking %s", event.MemberEmail, event.BookingID)
		return nil
	}

	auth := smtp.PlainAuth("", senderEmail, senderPassword, smtpHost)

	seats := strings.Join(event.SeatIDs, ", ")

	subject := fmt.Sprintf("Booking Confirmation - %s", event.BookingID)
	body := fmt.Sprintf(
		"Hi %s,\n\n"+
			"Your booking has been confirmed!\n\n"+
			"Booking ID: %s\n"+
			"Event: %s\n"+
			"Seats: %s\n"+
			"Total: %.2f THB\n\n"+
			"Thank you for your purchase!",
		event.MemberName,
		event.BookingID,
		event.EventName,
		seats,
		event.TotalAmount,
	)

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"UTF-8\"\r\n\r\n%s",
		senderEmail,
		event.MemberEmail,
		subject,
		body,
	)

	addr := fmt.Sprintf("%s:%s", smtpHost, smtpPort)
	return smtp.SendMail(addr, auth, senderEmail, []string{event.MemberEmail}, []byte(msg))
}
