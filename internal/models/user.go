package models

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

	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}
