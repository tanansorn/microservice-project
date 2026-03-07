package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/zensos/microservice-project/internal/common"
	"github.com/zensos/microservice-project/internal/database"
	"github.com/zensos/microservice-project/internal/middleware"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var db *gorm.DB

func main() {
	db = database.Connect()
	db.AutoMigrate(&Wallet{}, &Ledger{}, &Payment{})

	app := fiber.New()

	app.Use(middleware.RateLimiter(middleware.RateLimiterConfig{
		Max:        100,
		WindowSecs: 60,
	}))

	app.Get("/", func(c fiber.Ctx) error {
		return c.SendString("Payment Service")
	})

	app.Get("/health", func(c fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ok", "service": "payment"})
	})

	app.Get("/wallets/:member_id", func(c fiber.Ctx) error {
		memberID := c.Params("member_id")

		var wallet Wallet
		if err := db.Where("member_id = ?", memberID).First(&wallet).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "wallet not found"})
		}

		return c.JSON(wallet)
	})

	app.Post("/wallets/top-up", func(c fiber.Ctx) error {
		var req TopUpRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if req.MemberID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "member_id is required"})
		}
		if req.Amount <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "amount must be greater than 0"})
		}

		switch req.Method {
		case TopUpPromptpay, TopUpBanking, TopUpTrueWallet:
		default:
			return c.Status(400).JSON(fiber.Map{"error": "method must be one of: promptpay, banking, truewallet"})
		}

		var wallet Wallet
		var ledger Ledger

		txErr := db.Transaction(func(tx *gorm.DB) error {
			result := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("member_id = ?", req.MemberID).First(&wallet)

			if result.Error != nil {
				wallet = Wallet{
					MemberID: req.MemberID,
					Balance:  0,
				}
				if err := tx.Create(&wallet).Error; err != nil {
					return err
				}
			}

			wallet.Balance += req.Amount
			if err := tx.Save(&wallet).Error; err != nil {
				return err
			}

			ledger = Ledger{
				MemberID:     req.MemberID,
				Type:         LedgerTopUp,
				Method:       req.Method,
				Amount:       req.Amount,
				BalanceAfter: wallet.Balance,
				ReferenceID:  fmt.Sprintf("topup_%d", time.Now().UnixNano()),
			}
			return tx.Create(&ledger).Error
		})

		if txErr != nil {
			return c.Status(500).JSON(fiber.Map{"error": "failed to top up"})
		}

		return c.Status(201).JSON(fiber.Map{
			"wallet": wallet,
			"ledger": ledger,
		})
	})

	app.Post("/payments", func(c fiber.Ctx) error {
		var req PayBookingRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if req.MemberID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "member_id is required"})
		}
		if req.BookingID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "booking_id is required"})
		}
		if req.Amount <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "amount must be greater than 0"})
		}

		var payment Payment
		var ledger Ledger

		txErr := db.Transaction(func(tx *gorm.DB) error {
			var wallet Wallet
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("member_id = ?", req.MemberID).First(&wallet).Error; err != nil {
				return fmt.Errorf("wallet not found")
			}

			if wallet.Balance < req.Amount {
				return fmt.Errorf("insufficient balance")
			}

			wallet.Balance -= req.Amount
			if err := tx.Save(&wallet).Error; err != nil {
				return err
			}

			paymentID := fmt.Sprintf("pay_%d", time.Now().UnixNano())

			payment = Payment{
				PaymentID: paymentID,
				BookingID: req.BookingID,
				MemberID:  req.MemberID,
				Amount:    req.Amount,
				Status:    "confirmed",
			}
			if err := tx.Create(&payment).Error; err != nil {
				return err
			}

			ledger = Ledger{
				MemberID:     req.MemberID,
				Type:         LedgerPayment,
				Amount:       -req.Amount,
				BalanceAfter: wallet.Balance,
				ReferenceID:  paymentID,
			}
			return tx.Create(&ledger).Error
		})

		if txErr != nil {
			msg := txErr.Error()
			if msg == "wallet not found" {
				return c.Status(404).JSON(fiber.Map{"error": "wallet not found, please top up first"})
			}
			if msg == "insufficient balance" {
				return c.Status(400).JSON(fiber.Map{"error": "insufficient balance"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "failed to process payment"})
		}

		return c.Status(201).JSON(fiber.Map{
			"payment": payment,
			"ledger":  ledger,
		})
	})

	app.Get("/payments/:id", func(c fiber.Ctx) error {
		var payment Payment
		if err := db.Where("payment_id = ?", c.Params("id")).First(&payment).Error; err != nil {
			return c.Status(404).JSON(fiber.Map{"error": "payment not found"})
		}
		return c.JSON(payment)
	})

	app.Get("/wallets/:member_id/ledger", func(c fiber.Ctx) error {
		memberID := c.Params("member_id")

		var ledgers []Ledger
		db.Where("member_id = ?", memberID).Order("created_at desc").Find(&ledgers)

		return c.JSON(fiber.Map{
			"member_id": memberID,
			"ledger":    ledgers,
		})
	})

	app.Post("/payments/refund", func(c fiber.Ctx) error {
		var req RefundRequest
		if err := c.Bind().JSON(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "invalid request body"})
		}

		if req.MemberID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "member_id is required"})
		}
		if req.BookingID == "" {
			return c.Status(400).JSON(fiber.Map{"error": "booking_id is required"})
		}
		if req.Amount <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "amount must be greater than 0"})
		}

		var ledger Ledger

		txErr := db.Transaction(func(tx *gorm.DB) error {
			var payment Payment
			if err := tx.Where("booking_id = ? AND status = ?", req.BookingID, "confirmed").First(&payment).Error; err != nil {
				return fmt.Errorf("payment not found")
			}

			payment.Status = "refunded"
			if err := tx.Save(&payment).Error; err != nil {
				return err
			}

			var wallet Wallet
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
				Where("member_id = ?", req.MemberID).First(&wallet).Error; err != nil {
				return fmt.Errorf("wallet not found")
			}

			wallet.Balance += req.Amount
			if err := tx.Save(&wallet).Error; err != nil {
				return err
			}

			ledger = Ledger{
				MemberID:     req.MemberID,
				Type:         LedgerRefund,
				Amount:       req.Amount,
				BalanceAfter: wallet.Balance,
				ReferenceID:  payment.PaymentID,
			}
			return tx.Create(&ledger).Error
		})

		if txErr != nil {
			msg := txErr.Error()
			if msg == "payment not found" {
				return c.Status(404).JSON(fiber.Map{"error": "no confirmed payment found for this booking"})
			}
			if msg == "wallet not found" {
				return c.Status(404).JSON(fiber.Map{"error": "wallet not found"})
			}
			return c.Status(500).JSON(fiber.Map{"error": "failed to process refund"})
		}

		return c.JSON(fiber.Map{
			"message": "refund processed",
			"ledger":  ledger,
		})
	})

	consulClient, serviceID, err := common.RegisterService(common.ServiceConfig{
		Name: "payment",
		Port: 3004,
	})
	if err != nil {
		log.Printf("couldn't register with consul: %v", err)
	}

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

	log.Fatal(app.Listen(":3004"))
}
