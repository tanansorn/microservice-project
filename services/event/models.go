package main

import (
	"time"

	"gorm.io/gorm"
)

type Event struct {
	ID        uint           `json:"id" gorm:"primaryKey"`
	Name      string         `json:"name"`
	Price     float64        `json:"price"`
	Date      time.Time      `json:"date"`
	Venue     string         `json:"venue"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}
