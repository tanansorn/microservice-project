package main

import (
	"time"

	"gorm.io/gorm"
)

type Gender string

const (
	GenderMale   Gender = "male"
	GenderFemale Gender = "female"
	GenderOther  Gender = "other"
	GenderNA     Gender = "not_specified"
)

type IdentityType string

const (
	IdentityNationalID IdentityType = "national_id"
	IdentityPassport   IdentityType = "passport"
)

type Member struct {
	ID              uint           `json:"id" gorm:"primaryKey"`
	FirstName       string         `json:"first_name"`
	LastName        string         `json:"last_name"`
	Email           string         `json:"email" gorm:"uniqueIndex"`
	PasswordHash    string         `json:"-"`
	Gender          Gender         `json:"gender,omitempty"`
	BirthDay        int            `json:"birth_day,omitempty"`
	BirthMonth      int            `json:"birth_month,omitempty"`
	BirthYear       int            `json:"birth_year,omitempty"`
	PhoneCountry    string         `json:"phone_country,omitempty"`
	PhoneNumber     string         `json:"phone_number,omitempty"`
	AddressLine1    string         `json:"address_line1,omitempty"`
	AddressCountry  string         `json:"address_country,omitempty"`
	AddressProvince string         `json:"address_province,omitempty"`
	AddressDistrict string         `json:"address_district,omitempty"`
	PostalCode      string         `json:"postal_code,omitempty"`
	IdentityType    IdentityType   `json:"identity_type,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `json:"-" gorm:"index"`
}

type UpdateMemberRequest struct {
	FirstName   *string      `json:"first_name,omitempty"`
	LastName    *string      `json:"last_name,omitempty"`
	Gender      *Gender      `json:"gender,omitempty"`
	BirthDay    *int         `json:"birth_day,omitempty"`
	BirthMonth  *int         `json:"birth_month,omitempty"`
	BirthYear   *int         `json:"birth_year,omitempty"`
	AddressLine1    *string  `json:"address_line1,omitempty"`
	AddressCountry  *string  `json:"address_country,omitempty"`
	AddressProvince *string  `json:"address_province,omitempty"`
	AddressDistrict *string  `json:"address_district,omitempty"`
	PostalCode      *string  `json:"postal_code,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

type SignUpRequest struct {
	Email           string `json:"email"`
	Password        string `json:"password"`
	ConfirmPassword string `json:"confirm_password"`
	FirstName       string `json:"first_name"`
	LastName        string `json:"last_name"`
}

type SignInRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}
