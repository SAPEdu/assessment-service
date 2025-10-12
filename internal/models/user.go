package models

import (
	"time"

	"gorm.io/gorm"
)

type UserRole string
type Role = UserRole // Alias for compatibility

const (
	RoleStudent UserRole = "student"
	RoleTeacher UserRole = "teacher"
	RoleProctor UserRole = "proctor"
	RoleAdmin   UserRole = "admin"
)

type User struct {
	ID       string   `json:"id" gorm:"primaryKey;size:255"`
	FullName string   `json:"full_name" gorm:"not null;size:100"`
	Email    string   `json:"email" gorm:"uniqueIndex;not null;size:255"`
	Role     UserRole `json:"role" gorm:"-"`

	// Profile info
	AvatarURL *string `json:"avatar_url" gorm:"size:500"`

	// Settings

	// Status
	EmailVerified bool `json:"email_verified" gorm:"default:false"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`
}

func (User) TableName() string {
	return "users"
}
