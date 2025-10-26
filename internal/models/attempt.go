package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type AttemptStatus string

const (
	AttemptInProgress AttemptStatus = "in_progress"
	AttemptCompleted  AttemptStatus = "completed"
	AttemptAbandoned  AttemptStatus = "abandoned"
	AttemptTimeOut    AttemptStatus = "timeout"
)

const (
	AttemptEndReasonTimeout = "time_out"
)

type AssessmentAttempt struct {
	ID            uint          `json:"id" gorm:"primaryKey"`
	AssessmentID  uint          `json:"assessment_id" gorm:"not null;index"`
	StudentID     string        `json:"student_id" gorm:"not null;index;size:255"`
	AttemptNumber int           `json:"attempt_number" gorm:"not null"`
	Status        AttemptStatus `json:"status" gorm:"default:in_progress;index"`

	// Timing
	StartedAt     *time.Time `json:"started_at"`
	EndedAt       *time.Time `json:"ended_at"`
	CompletedAt   *time.Time `json:"completed_at"`
	TimeSpent     int        `json:"time_spent"`     // seconds
	TimeRemaining int        `json:"time_remaining"` // seconds

	// Scoring
	Score      float64 `json:"score"`
	MaxScore   int     `json:"max_score"`
	Percentage float64 `json:"percentage"`
	Passed     bool    `json:"passed"`
	IsGraded   bool    `json:"is_graded"`

	// Progress tracking
	CurrentQuestionIndex int  `json:"current_question_index"`
	QuestionsAnswered    int  `json:"questions_answered"`
	TotalQuestions       int  `json:"total_questions"`
	IsReview             bool `json:"is_review"` // Review mode before submit

	// Metadata
	IPAddress   *string        `json:"ip_address" gorm:"size:45"`
	UserAgent   *string        `json:"user_agent" gorm:"type:text"`
	SessionData datatypes.JSON `json:"session_data" gorm:"type:jsonb"` // Browser info, screen resolution, etc.
	EndReason   *string        `json:"end_reason" gorm:"type:text"`    // e.g., "time_out", "abandoned", "completed"

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Assessment       Assessment        `json:"assessment" gorm:"foreignKey:AssessmentID"`
	Student          User              `json:"student" gorm:"foreignKey:StudentID"`
	Answers          []StudentAnswer   `json:"answers" gorm:"foreignKey:AttemptID"`
	ProctoringEvents []ProctoringEvent `json:"proctoring_events" gorm:"foreignKey:AttemptID"`

	// Unique constraint per student per assessment
	gorm.Model `gorm:"uniqueIndex:idx_student_assessment_attempt"`
}

type StudentAnswer struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	AttemptID  uint `json:"attempt_id" gorm:"not null;index"`
	QuestionID uint `json:"question_id" gorm:"not null;index"`

	// Answer content (polymorphic based on question type)
	Answer datatypes.JSON `json:"answer" gorm:"type:jsonb"`

	// Grading
	Score     float64    `json:"score"`
	MaxScore  int        `json:"max_score"`
	IsCorrect *bool      `json:"is_correct"`                // null for essay/manual grading
	GradedBy  *string    `json:"graded_by" gorm:"size:255"` // Teacher ID for manual grading
	GradedAt  *time.Time `json:"graded_at"`
	Feedback  *string    `json:"feedback" gorm:"type:text"`

	// Timing
	TimeSpent       int        `json:"time_spent"` // seconds
	FirstAnsweredAt *time.Time `json:"first_answered_at"`
	LastModifiedAt  *time.Time `json:"last_modified_at"`

	// Metadata
	AnswerHistory datatypes.JSON `json:"answer_history" gorm:"type:jsonb"` // Track changes
	Flagged       bool           `json:"flagged"`                          // Student flagged for review
	IsGraded      bool           `json:"is_graded"`                        // Whether the answer has been graded

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Attempt  AssessmentAttempt `json:"attempt" gorm:"foreignKey:AttemptID"`
	Question Question          `json:"question" gorm:"foreignKey:QuestionID"`
	Grader   *User             `json:"grader" gorm:"foreignKey:GradedBy"`
}
