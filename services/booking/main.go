package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/sony/gobreaker/v2"

	"github.com/gofiber/fiber/v3"
	"github.com/zensos/microservice-project/internal/circuitbreaker"
	"github.com/zensos/microservice-project/internal/common"
	"github.com/zensos/microservice-project/internal/database"
	"github.com/zensos/microservice-project/internal/middleware"
	"github.com/zensos/microservice-project/internal/rabbitmq"
	"gorm.io/gorm"
)

var (
	db   *gorm.DB
	mqCh *amqp.Channel

	eventCB   *gobreaker.CircuitBreaker[circuitbreaker.BreakerResponse]
	memberCB  *gobreaker.CircuitBreaker[circuitbreaker.BreakerResponse]
	paymentCB *gobreaker.CircuitBreaker[circuitbreaker.BreakerResponse]
)

func main() {
	db = database.Connect()
	db.AutoMigrate(&Booking{}, &BookingSeat{})

	db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS idx_booking_seats_event_seat ON booking_seats (event_id, seat_id) WHERE deleted_at IS NULL")

	mqConn := rabbitmq.Connect()
	defer mqConn.Close()

	var err error
	mqCh, err = mqConn.Channel()
	if err != nil {
		log.Fatalf("failed to open rabbitmq channel: %v", err)
	}
	defer mqCh.Close()

	rabbitmq.DeclareQueue(mqCh, "booking.confirmed")

	eventCB = circuitbreaker.NewBreaker("event-service")
	memberCB = circuitbreaker.NewBreaker("member-service")
	paymentCB = circuitbreaker.NewBreaker("payment-service")

	app := fiber.New()

	app.Use(middleware.RateLimiter(middleware.RateLimiterConfig{
		Max:        100,
		WindowSecs: 60,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Booking Service")
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "booking"})
	})

	consulClient, serviceID, err := common.RegisterService(common.ServiceConfig{
		Name: "booking",
		Port: 3001,
	})
	if err != nil {
		log.Printf("couldn't register with consul: %v", err)
	}

	app.Post("/bookings", func(c fiber.Ctx) error {
		if consulClient == nil {
			return c.Status(503).JSON(fiber.Map{"error": "service discovery is not available right now"})
		}

		var req CreateBookingRequest
		if err := c.Bind().Body(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid json body"})
		}

		if req.EventID <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "event_id must be > 0"})
		}
		if req.MemberID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "member_id is required"})
		}
		if len(req.SeatIDs) == 0 {
			return c.Status(400).JSON(fiber.Map{"error": "seat_ids must not be empty"})
		}

		seen := map[string]bool{}
		for _, seat := range req.SeatIDs {
			if seat == "" {
				return c.Status(400).JSON(fiber.Map{"error": "seat_id must not be empty"})
			}
			if seen[seat] {
				return c.Status(400).JSON(fiber.Map{"error": "seat_ids must not contain duplicates"})
			}
			seen[seat] = true
		}

		eventAddr, err := common.DiscoverService(consulClient, "event")
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't find the event service: %v", err)})
		}

		eventURL := fmt.Sprintf("http://%s/events/%d", eventAddr, req.EventID)
		eventReq, _ := http.NewRequest("GET", eventURL, nil)
		eventResp, err := circuitbreaker.Do(eventCB, eventReq)
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				return c.Status(503).JSON(fiber.Map{"error": "event service is temporarily unavailable"})
			}
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't reach the event service: %v", err)})
		}

		if eventResp.StatusCode == 404 {
			return c.Status(404).JSON(fiber.Map{"error": "event not found"})
		}
		if eventResp.StatusCode != 200 {
			return c.Status(502).JSON(fiber.Map{"error": "event service error"})
		}

		var eventData map[string]any
		if err := json.Unmarshal(eventResp.Body, &eventData); err != nil {
			return c.Status(502).JSON(fiber.Map{"error": "got a bad response from the event service"})
		}

		price, _ := eventData["price"].(float64)
		totalAmount := price * float64(len(req.SeatIDs))
		eventName, _ := eventData["name"].(string)

		memberAddr, err := common.DiscoverService(consulClient, "member")
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't find the member service: %v", err)})
		}

		memberURL := fmt.Sprintf("http://%s/members/%s", memberAddr, req.MemberID)
		memberReq, _ := http.NewRequest("GET", memberURL, nil)
		memberResp, err := circuitbreaker.Do(memberCB, memberReq)
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				return c.Status(503).JSON(fiber.Map{"error": "member service is temporarily unavailable"})
			}
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't reach the member service: %v", err)})
		}

		if memberResp.StatusCode == 404 {
			return c.Status(404).JSON(fiber.Map{"error": "user not found"})
		}
		if memberResp.StatusCode != 200 {
			return c.Status(502).JSON(fiber.Map{"error": "member service error"})
		}

		var memberData map[string]any
		json.Unmarshal(memberResp.Body, &memberData)

		memberEmail, _ := memberData["email"].(string)
		firstName, _ := memberData["first_name"].(string)
		lastName, _ := memberData["last_name"].(string)
		memberName := strings.TrimSpace(firstName + " " + lastName)

		paymentAddr, err := common.DiscoverService(consulClient, "payment")
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't find the payment service: %v", err)})
		}

		bookingID := fmt.Sprintf("book_%d", time.Now().UnixNano())

		payBody, _ := json.Marshal(map[string]any{
			"booking_id": bookingID,
			"member_id":  req.MemberID,
			"amount":     totalAmount,
		})

		payURL := fmt.Sprintf("http://%s/payments", paymentAddr)
		payReq, _ := http.NewRequest("POST", payURL, bytes.NewReader(payBody))
		payReq.Header.Set("Content-Type", "application/json")
		payResp, err := circuitbreaker.Do(paymentCB, payReq)
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				return c.Status(503).JSON(fiber.Map{"error": "payment service is temporarily unavailable"})
			}
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't reach the payment service: %v", err)})
		}

		if payResp.StatusCode != 201 {
			var payErr map[string]any
			json.Unmarshal(payResp.Body, &payErr)
			errMsg := "payment failed"
			if msg, ok := payErr["error"].(string); ok {
				errMsg = msg
			}
			return c.Status(payResp.StatusCode).JSON(fiber.Map{"error": errMsg})
		}

		txErr := db.Transaction(func(tx *gorm.DB) error {
			booking := Booking{
				BookingID:   bookingID,
				EventID:     req.EventID,
				MemberID:    req.MemberID,
				TotalAmount: totalAmount,
				Status:      "CONFIRMED",
			}
			if err := tx.Create(&booking).Error; err != nil {
				return err
			}

			for _, seatID := range req.SeatIDs {
				seat := BookingSeat{
					BookingID: bookingID,
					EventID:   req.EventID,
					SeatID:    seatID,
				}
				if err := tx.Create(&seat).Error; err != nil {
					return err
				}
			}

			return nil
		})

		if txErr != nil {
			refundBody, _ := json.Marshal(map[string]any{
				"booking_id": bookingID,
				"member_id":  req.MemberID,
				"amount":     totalAmount,
			})
			refundURL := fmt.Sprintf("http://%s/payments/refund", paymentAddr)
			refundReq, _ := http.NewRequest("POST", refundURL, bytes.NewReader(refundBody))
			refundReq.Header.Set("Content-Type", "application/json")
			circuitbreaker.Do(paymentCB, refundReq)

			if isDuplicateKeyError(txErr) {
				return c.Status(409).JSON(fiber.Map{"error": "one or more seats are already reserved"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "failed to create booking"})
		}

		mqEvent, _ := json.Marshal(map[string]any{
			"booking_id":   bookingID,
			"member_id":    req.MemberID,
			"member_email": memberEmail,
			"member_name":  memberName,
			"event_name":   eventName,
			"seat_ids":     req.SeatIDs,
			"total_amount": totalAmount,
		})
		if err := rabbitmq.Publish(mqCh, "booking.confirmed", mqEvent); err != nil {
			log.Printf("failed to publish booking event to rabbitmq: %v", err)
		}

		return c.Status(201).JSON(fiber.Map{
			"booking_id":   bookingID,
			"event":        eventData,
			"member_id":    req.MemberID,
			"seat_ids":     req.SeatIDs,
			"total_amount": totalAmount,
			"status":       "CONFIRMED",
		})
	})

	app.Post("/bookings/:booking_id/cancel", func(c fiber.Ctx) error {
		if consulClient == nil {
			return c.Status(503).JSON(fiber.Map{"error": "service discovery is not available right now"})
		}

		bookingID := c.Params("booking_id")

		var booking Booking
		if err := db.Preload("Seats").Where("booking_id = ?", bookingID).First(&booking).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "booking not found"})
		}

		if booking.Status == "CANCELLED" {
			return c.Status(400).JSON(fiber.Map{"error": "booking is already cancelled"})
		}

		paymentAddr, err := common.DiscoverService(consulClient, "payment")
		if err != nil {
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't find the payment service: %v", err)})
		}

		refundBody, _ := json.Marshal(map[string]any{
			"booking_id": bookingID,
			"member_id":  booking.MemberID,
			"amount":     booking.TotalAmount,
		})

		refundURL := fmt.Sprintf("http://%s/payments/refund", paymentAddr)
		refundReq, _ := http.NewRequest("POST", refundURL, bytes.NewReader(refundBody))
		refundReq.Header.Set("Content-Type", "application/json")
		refundResp, err := circuitbreaker.Do(paymentCB, refundReq)
		if err != nil {
			if errors.Is(err, gobreaker.ErrOpenState) {
				return c.Status(503).JSON(fiber.Map{"error": "payment service is temporarily unavailable"})
			}
			return c.Status(502).JSON(fiber.Map{"error": fmt.Sprintf("couldn't reach the payment service: %v", err)})
		}

		if refundResp.StatusCode != 200 {
			var refundErr map[string]any
			json.Unmarshal(refundResp.Body, &refundErr)
			errMsg := "refund failed"
			if msg, ok := refundErr["error"].(string); ok {
				errMsg = msg
			}
			return c.Status(refundResp.StatusCode).JSON(fiber.Map{"error": errMsg})
		}

		db.Model(&booking).Update("status", "CANCELLED")

		for _, seat := range booking.Seats {
			db.Delete(&seat)
		}

		return c.JSON(fiber.Map{
			"message":    "booking cancelled and refunded",
			"booking_id": bookingID,
			"refunded":   booking.TotalAmount,
		})
	})

	app.Get("/users/:member_id/tickets", func(c fiber.Ctx) error {
		memberID := c.Params("member_id")
		if memberID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "member_id is required"})
		}

		var bookings []Booking
		db.Preload("Seats").Where("member_id = ?", memberID).Find(&bookings)

		return c.JSON(fiber.Map{
			"member_id": memberID,
			"bookings":  bookings,
		})
	})

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		if consulClient != nil {
			if err := common.DeregisterService(consulClient, serviceID); err != nil {
				log.Printf("couldn't deregister from consul: %v", err)
			}
		}
		if err := app.Shutdown(); err != nil {
			log.Printf("something went wrong while shutting down: %v", err)
		}
	}()

	log.Fatal(app.Listen(":3001"))
}

func isDuplicateKeyError(err error) bool {
	var pgErr interface{ SQLState() string }
	if errors.As(err, &pgErr) {
		return pgErr.SQLState() == "23505"
	}
	return false
}
