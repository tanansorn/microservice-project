package main

import (
	"time"

	"gorm.io/gorm"
)

type Booking struct {
	ID          uint           `json:"id" gorm:"primaryKey"`
	BookingID   string         `json:"booking_id" gorm:"uniqueIndex"`
	EventID     uint           `json:"event_id" gorm:"index"`
	MemberID    string         `json:"member_id" gorm:"index"`
	TotalAmount float64        `json:"total_amount"`
	Status      string         `json:"status"`
	Seats       []BookingSeat  `json:"seats" gorm:"foreignKey:BookingID;references:BookingID"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `json:"-" gorm:"index"`
}

type BookingSeat struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	BookingID string         `json:"booking_id" gorm:"index"`
	EventID   uint           `json:"event_id"`
	SeatID    string         `json:"seat_id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

type CreateBookingRequest struct {
	EventID  uint     `json:"event_id"`
	MemberID string   `json:"member_id"`
	SeatIDs  []string `json:"seat_ids"`
}
