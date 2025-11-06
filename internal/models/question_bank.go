package models

import (
	"time"
)

type QuestionBank struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name" gorm:"not null;size:200" validate:"required,max=200"`
	Description *string `json:"description" gorm:"type:text"`

	// Access control
	IsPublic bool `json:"is_public" gorm:"default:false"`
	IsShared bool `json:"is_shared" gorm:"default:false"`

	// Metadata
	CreatedBy string    `json:"created_by" gorm:"not null;index;size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Questions  []Question          `json:"questions" gorm:"many2many:question_bank_questions"`
	Creator    User                `json:"creator" gorm:"foreignKey:CreatedBy"`
	SharedWith []QuestionBankShare `json:"shared_with" gorm:"foreignKey:BankID"`

	// Statistics
	QuestionCount int `json:"question_count" gorm:"-"`
	UsageCount    int `json:"usage_count" gorm:"-"`
}

type QuestionBankShare struct {
	ID     uint   `json:"id" gorm:"primaryKey"`
	BankID uint   `json:"bank_id" gorm:"not null;index"`
	UserID string `json:"user_id" gorm:"not null;index;size:255"`

	// Permissions
	CanView   bool `json:"can_view" gorm:"default:true"`
	CanEdit   bool `json:"can_edit" gorm:"default:false"`
	CanDelete bool `json:"can_delete" gorm:"default:false"`

	SharedAt time.Time `json:"shared_at"`
	SharedBy string    `json:"shared_by" gorm:"not null;size:255"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Bank   QuestionBank `json:"bank" gorm:"foreignKey:BankID"`
	User   User         `json:"user" gorm:"foreignKey:UserID"`
	Sharer User         `json:"sharer" gorm:"foreignKey:SharedBy"`
}
