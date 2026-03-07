package middleware

import (
	"time"

	"github.com/gofiber/fiber/v3"
)

type RateLimiterConfig struct {
	Max        int
	WindowSecs int
}

type client struct {
	count    int
	expireAt time.Time
}

func RateLimiter(cfg RateLimiterConfig) fiber.Handler {
	if cfg.Max == 0 {
		cfg.Max = 60
	}
	if cfg.WindowSecs == 0 {
		cfg.WindowSecs = 60
	}

	clients := make(map[string]*client)
	window := time.Duration(cfg.WindowSecs) * time.Second

	go func() {
		for {
			time.Sleep(window)
			now := time.Now()
			for k, c := range clients {
				if now.After(c.expireAt) {
					delete(clients, k)
				}
			}
		}
	}()

	return func(c fiber.Ctx) error {
		ip := c.IP()

		cl, exists := clients[ip]
		if !exists || time.Now().After(cl.expireAt) {
			clients[ip] = &client{
				count:    1,
				expireAt: time.Now().Add(window),
			}
			return c.Next()
		}

		cl.count++
		if cl.count > cfg.Max {
			return c.Status(429).JSON(fiber.Map{
				"error": "too many requests, please try again later",
			})
		}

		return c.Next()
	}
}
