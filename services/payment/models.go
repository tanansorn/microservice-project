package main

import (
	"time"

	"gorm.io/gorm"
)

type Wallet struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	MemberID  string         `json:"member_id" gorm:"uniqueIndex"`
	Balance   float64        `json:"balance"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type LedgerType string

const (
	LedgerTopUp   LedgerType = "top_up"
	LedgerPayment LedgerType = "payment"
	LedgerRefund  LedgerType = "refund"
)

type TopUpMethod string

const (
	TopUpPromptpay   TopUpMethod = "promptpay"
	TopUpBanking     TopUpMethod = "banking"
	TopUpTrueWallet  TopUpMethod = "truewallet"
)

type Ledger struct {
	ID           uint           `json:"id" gorm:"primaryKey"`
	MemberID     string         `json:"member_id" gorm:"index"`
	Type         LedgerType     `json:"type"`
	Method       TopUpMethod    `json:"method,omitempty"`
	Amount       float64        `json:"amount"`
	BalanceAfter float64        `json:"balance_after"`
	ReferenceID  string         `json:"reference_id"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `json:"-" gorm:"index"`
}

type Payment struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	PaymentID string         `json:"payment_id" gorm:"uniqueIndex"`
	BookingID string         `json:"booking_id" gorm:"index"`
	MemberID  string         `json:"member_id" gorm:"index"`
	Amount    float64        `json:"amount"`
	Status    string         `json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type TopUpRequest struct {
	MemberID string      `json:"member_id"`
	Amount   float64     `json:"amount"`
	Method   TopUpMethod `json:"method"`
}

type PayBookingRequest struct {
	BookingID string  `json:"booking_id"`
	MemberID  string  `json:"member_id"`
	Amount    float64 `json:"amount"`
}

type RefundRequest struct {
	BookingID string  `json:"booking_id"`
	MemberID  string  `json:"member_id"`
	Amount    float64 `json:"amount"`
}
