package main

import (
	"strconv"

	"github.com/gofiber/fiber/v3"
)

func getMemberProfile(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	var member Member
	if err := db.First(&member, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Member with ID " + c.Params("id") + " not found")
	}

	return c.JSON(member)
}

func updateMemberProfile(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	var member Member
	if err := db.First(&member, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Member with ID " + c.Params("id") + " not found")
	}

	var req UpdateMemberRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
	}

	updates := map[string]any{}
	if req.FirstName != nil {
		updates["first_name"] = *req.FirstName
	}
	if req.LastName != nil {
		updates["last_name"] = *req.LastName
	}
	if req.Gender != nil {
		updates["gender"] = *req.Gender
	}
	if req.BirthDay != nil {
		updates["birth_day"] = *req.BirthDay
	}
	if req.BirthMonth != nil {
		updates["birth_month"] = *req.BirthMonth
	}
	if req.BirthYear != nil {
		updates["birth_year"] = *req.BirthYear
	}
	if req.AddressLine1 != nil {
		updates["address_line1"] = *req.AddressLine1
	}
	if req.AddressCountry != nil {
		updates["address_country"] = *req.AddressCountry
	}
	if req.AddressProvince != nil {
		updates["address_province"] = *req.AddressProvince
	}
	if req.AddressDistrict != nil {
		updates["address_district"] = *req.AddressDistrict
	}
	if req.PostalCode != nil {
		updates["postal_code"] = *req.PostalCode
	}

	if len(updates) > 0 {
		db.Model(&member).Updates(updates)
	}

	db.First(&member, id)
	return c.JSON(member)
}

func changePassword(c fiber.Ctx) error {
	id, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	var member Member
	if err := db.First(&member, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).SendString("Member with ID " + c.Params("id") + " not found")
	}

	var req ChangePasswordRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
	}

	if member.PasswordHash != req.CurrentPassword {
		return c.Status(fiber.StatusBadRequest).SendString("Current password is incorrect")
	}
	if req.NewPassword != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).SendString("New password and confirm password do not match")
	}

	db.Model(&member).Update("password_hash", req.NewPassword)
	return c.JSON(fiber.Map{"message": "Password changed successfully"})
}

func signup(c fiber.Ctx) error {
	var req SignUpRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	if req.Password != req.ConfirmPassword {
		return c.Status(fiber.StatusBadRequest).SendString("Password and confirm password do not match")
	}

	var existing Member
	if err := db.Where("email = ?", req.Email).First(&existing).Error; err == nil {
		return c.Status(fiber.StatusBadRequest).SendString("Email already exists")
	}

	member := Member{
		FirstName:    req.FirstName,
		LastName:     req.LastName,
		Email:        req.Email,
		PasswordHash: req.Password,
	}
	if err := db.Create(&member).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to create member")
	}

	return c.Status(fiber.StatusCreated).JSON(member)
}

func signin(c fiber.Ctx) error {
	var req SignInRequest
	if err := c.Bind().JSON(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString(err.Error())
	}

	var member Member
	if err := db.Where("email = ? AND password_hash = ?", req.Email, req.Password).First(&member).Error; err != nil {
		return c.Status(fiber.StatusUnauthorized).SendString("Invalid email or password")
	}

	return c.JSON(fiber.Map{
		"message": "Sign in successful",
		"member":  member,
	})
}