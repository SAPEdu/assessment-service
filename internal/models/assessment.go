package models

import (
	"time"

	"gorm.io/gorm"
)

type AssessmentStatus string

const (
	StatusDraft    AssessmentStatus = "Draft"
	StatusActive   AssessmentStatus = "Active"
	StatusExpired  AssessmentStatus = "Expired"
	StatusArchived AssessmentStatus = "Archived"
)

type Assessment struct {
	ID           uint             `json:"id" gorm:"primaryKey"`
	Title        string           `json:"title" gorm:"not null;size:200;index" validate:"required,min=1,max=200"`
	Description  *string          `json:"description" gorm:"type:text" validate:"omitempty,max=1000"`
	Duration     int              `json:"duration" gorm:"not null" validate:"required,min=5,max=300"`
	Status       AssessmentStatus `json:"status" gorm:"default:Draft;index" validate:"omitempty,oneof=Draft Active Expired Archived"`
	PassingScore int              `json:"passing_score" gorm:"not null" validate:"required,min=0,max=100"`
	MaxAttempts  int              `json:"max_attempts" gorm:"default:1" validate:"min=1,max=10"`
	TimeWarning  int              `json:"time_warning" gorm:"default:300"` // Warning time in seconds
	DueDate      *time.Time       `json:"due_date"`

	// Metadata
	CreatedBy string         `json:"created_by" gorm:"not null;index;size:255"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"-" gorm:"index"`

	// Version control
	Version int `json:"version" gorm:"default:1"`

	// Relations
	Settings  AssessmentSettings   `json:"settings" gorm:"foreignKey:AssessmentID"`
	Questions []AssessmentQuestion `json:"questions" gorm:"foreignKey:AssessmentID"`
	Attempts  []AssessmentAttempt  `json:"attempts" gorm:"foreignKey:AssessmentID"`
	Creator   User                 `json:"creator" gorm:"foreignKey:CreatedBy"`

	// Computed fields (not stored)
	QuestionsCount int     `json:"questions_count" gorm:"-"`
	TotalPoints    int     `json:"total_points" gorm:"-"`
	AttemptCount   int     `json:"attempt_count" gorm:"-"`
	AvgScore       float64 `json:"avg_score" gorm:"-"`
}

type AssessmentSettings struct {
	AssessmentID uint      `json:"assessment_id" gorm:"primaryKey;constraint:OnUpdate:CASCADE,OnDelete:CASCADE"`
	CreatedAt    time.Time `json:"created_at" gorm:"not null"`
	UpdatedAt    time.Time `json:"updated_at" gorm:"not null"`

	// Question Display Settings
	RandomizeQuestions bool `json:"randomize_questions" gorm:"not null;default:false;comment:Randomize question order"`
	RandomizeOptions   bool `json:"randomize_options" gorm:"not null;default:false;comment:Randomize answer options"`
	ShowProgressBar    bool `json:"show_progress_bar" gorm:"not null;default:true;comment:Show progress indicator"`

	// Proctoring Settings
	RequireWebcam               bool `json:"require_webcam" gorm:"not null;default:false;comment:Require webcam for proctoring"`
	PreventTabSwitching         bool `json:"prevent_tab_switching" gorm:"not null;default:false;comment:Prevent switching browser tabs"`
	PreventRightClick           bool `json:"prevent_right_click" gorm:"not null;default:false;comment:Disable right-click context menu"`
	PreventCopyPaste            bool `json:"prevent_copy_paste" gorm:"not null;default:false;comment:Disable copy/paste functionality"`
	RequireIdentityVerification bool `json:"require_identity_verification" gorm:"not null;default:false;comment:Require identity verification"`
	RequireFullScreen           bool `json:"require_full_screen" gorm:"not null;default:false;comment:Force fullscreen mode"`

	// Accessibility Settings
	AllowScreenReader  bool `json:"allow_screen_reader" gorm:"not null;default:false;comment:Enable screen reader support"`
	FontSizeAdjustment int  `json:"font_size_adjustment" gorm:"not null;default:0;check:font_size_adjustment >= -2 AND font_size_adjustment <= 2;comment:Font size adjustment (-2 to +2)"`
	HighContrastMode   bool `json:"high_contrast_mode" gorm:"not null;default:false;comment:Enable high contrast display mode"`

	// Relations
	// Assessment Assessment `json:"assessment" gorm:"foreignKey:AssessmentID;references:ID"`
}

func (Assessment) TableName() string {
	return "assessments"
}

func (AssessmentSettings) TableName() string {
	return "assessment_settings"
}
