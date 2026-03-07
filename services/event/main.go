package main

import (
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/gofiber/fiber/v3"
	"github.com/zensos/microservice-project/internal/common"
	"github.com/zensos/microservice-project/internal/database"
	"github.com/zensos/microservice-project/internal/middleware"
	"gorm.io/gorm"
)

var db *gorm.DB

func main() {
	db = database.Connect()
	db.AutoMigrate(&Event{})

	app := fiber.New()

	app.Use(middleware.RateLimiter(middleware.RateLimiterConfig{
		Max:        100,
		WindowSecs: 60,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Event Service")
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "event"})
	})

	app.Get("/events", func(c fiber.Ctx) error {
		var events []Event
		db.Find(&events)
		return c.JSON(events)
	})

	app.Get("/events/:id", func(c fiber.Ctx) error {
		id, err := strconv.Atoi(c.Params("id"))
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid event id"})
		}

		var event Event
		if err := db.First(&event, id).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "event not found"})
		}

		return c.JSON(event)
	})

	app.Post("/events", func(c fiber.Ctx) error {
		var event Event
		if err := c.Bind().JSON(&event); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if err := db.Create(&event).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to create event"})
		}

		return c.Status(201).JSON(event)
	})

	consulClient, serviceID, err := common.RegisterService(common.ServiceConfig{
		Name: "event",
		Port: 3002,
	})
	if err != nil {
		log.Printf("Warning: failed to register with Consul: %v", err)
	}

	go func() {
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
		<-quit

		if consulClient != nil {
			if err := common.DeregisterService(consulClient, serviceID); err != nil {
				log.Printf("Warning: failed to deregister from Consul: %v", err)
			}
		}
		if err := app.Shutdown(); err != nil {
			log.Printf("Warning: server shutdown error: %v", err)
		}
	}()

	log.Fatal(app.Listen(":3002"))
}
