package main

import (
	"log"
	"os"
	"os/signal"
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
	db.AutoMigrate(&Member{})

	app := fiber.New()

	app.Use(middleware.RateLimiter(middleware.RateLimiterConfig{
		Max:        100,
		WindowSecs: 60,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Member Service")
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "member"})
	})

	consulClient, serviceID, err := common.RegisterService(common.ServiceConfig{
		Name: "member",
		Port: 3003,
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

	app.Get("/members/:id", getMemberProfile)
	app.Patch("/members/:id", updateMemberProfile)
	app.Post("/members/:id/change-password", changePassword)
	app.Post("/auth/signup", signup)
	app.Post("/auth/signin", signin)

	log.Fatal(app.Listen(":3003"))
}
