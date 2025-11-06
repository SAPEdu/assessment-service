package models

import (
	"time"

	"gorm.io/datatypes"
)

type QuestionType string

const (
	MultipleChoice QuestionType = "multiple_choice"
	TrueFalse      QuestionType = "true_false"
	Essay          QuestionType = "essay"
	FillInBlank    QuestionType = "fill_blank"
	Matching       QuestionType = "matching"
	Ordering       QuestionType = "ordering"
	ShortAnswer    QuestionType = "short_answer"
)

type DifficultyLevel string

const (
	DifficultyEasy   DifficultyLevel = "easy"
	DifficultyMedium DifficultyLevel = "medium"
	DifficultyHard   DifficultyLevel = "hard"
)

type Question struct {
	ID        uint         `json:"id" gorm:"primaryKey"`
	Type      QuestionType `json:"type" gorm:"not null;index"`
	Text      string       `json:"text" gorm:"type:text;not null" validate:"required"`
	Points    int          `json:"points" gorm:"default:10" validate:"min=1,max=100"` // Suggested/default points. Actual points determined by AssessmentQuestion.Points when added to assessment.
	TimeLimit *int         `json:"time_limit"`                                        // DEPRECATED: Not used in timing logic. Use Assessment.Duration instead. Kept for backward compatibility.
	Order     int          `json:"order" gorm:"default:0"`

	// Content stored as JSONB for flexibility
	Content datatypes.JSON `json:"content" gorm:"type:jsonb"`
	Answer  datatypes.JSON `json:"answer" gorm:"type:jsonb"` // Correct answer for the question

	// Categorization
	CategoryID *uint           `json:"category_id" gorm:"index"`
	Difficulty DifficultyLevel `json:"difficulty" gorm:"default:medium;index"`
	Tags       datatypes.JSON  `json:"tags" gorm:"type:jsonb"` // []string

	// Metadata
	Explanation *string   `json:"explanation" gorm:"type:text"`
	CreatedBy   string    `json:"created_by" gorm:"not null;index;size:255"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Relations
	Category    *QuestionCategory    `json:"category" gorm:"foreignKey:CategoryID"`
	Attachments []QuestionAttachment `json:"attachments" gorm:"foreignKey:QuestionID"`
	Creator     User                 `json:"creator" gorm:"foreignKey:CreatedBy"`

	// Statistics (computed)
	UsageCount       int     `json:"usage_count" gorm:"-"`
	AvgScore         float64 `json:"avg_score" gorm:"-"`
	DifficultyActual float64 `json:"difficulty_actual" gorm:"-"` // Calculated from attempts
}

// AssessmentQuestion - Many-to-many relationship with custom fields
type AssessmentQuestion struct {
	ID           uint `json:"id" gorm:"primaryKey"`
	AssessmentID uint `json:"assessment_id" gorm:"not null;index"`
	QuestionID   uint `json:"question_id" gorm:"not null;index"`

	// Override settings
	Order     int  `json:"order" gorm:"not null"`
	Points    *int `json:"points"`     // REQUIRED when adding to assessment. Overrides Question.Points. Total points across all questions must not exceed 100.
	TimeLimit *int `json:"time_limit"` // DEPRECATED: Not used in timing logic. Use Assessment.Duration instead. Kept for backward compatibility.
	Required  bool `json:"required" gorm:"default:true"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Assessment Assessment `json:"assessment" gorm:"foreignKey:AssessmentID"`
	Question   Question   `json:"question" gorm:"foreignKey:QuestionID"`

	// Unique constraint
	// gorm.Model `gorm:"uniqueIndex:idx_assessment_question"`
}

type QuestionCategory struct {
	ID          uint    `json:"id" gorm:"primaryKey"`
	Name        string  `json:"name" gorm:"not null;size:100" validate:"required,max=100"`
	Description *string `json:"description" gorm:"type:text"`
	Color       string  `json:"color" gorm:"size:7;default:#3B82F6"` // Hex color
	Icon        *string `json:"icon" gorm:"size:50"`

	// Hierarchy support
	ParentID *uint  `json:"parent_id" gorm:"index"`
	Level    int    `json:"level" gorm:"default:0"`
	Path     string `json:"path" gorm:"size:500"` // "/parent/child/grandchild"

	// Metadata
	CreatedBy string    `json:"created_by" gorm:"not null;index;size:255"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// Relations
	Parent    *QuestionCategory  `json:"parent" gorm:"foreignKey:ParentID"`
	Children  []QuestionCategory `json:"children" gorm:"foreignKey:ParentID"`
	Questions []Question         `json:"questions" gorm:"foreignKey:CategoryID"`
	Creator   User               `json:"creator" gorm:"foreignKey:CreatedBy"`

	// Statistics
	QuestionCount int `json:"question_count" gorm:"-"`
}

type QuestionAttachment struct {
	ID         uint `json:"id" gorm:"primaryKey"`
	QuestionID uint `json:"question_id" gorm:"not null;index"`

	FileName string `json:"file_name" gorm:"not null;size:255"`
	FileType string `json:"file_type" gorm:"not null;size:50"`
	FileSize int64  `json:"file_size" gorm:"not null"`
	MimeType string `json:"mime_type" gorm:"not null;size:100"`

	// Storage info
	StoragePath  string  `json:"storage_path" gorm:"not null;size:500"`
	URL          string  `json:"url" gorm:"not null;size:500"`
	ThumbnailURL *string `json:"thumbnail_url" gorm:"size:500"`

	// Metadata
	Alt     *string `json:"alt" gorm:"size:255"` // For images
	Caption *string `json:"caption" gorm:"type:text"`
	Order   int     `json:"order" gorm:"default:0"`

	CreatedAt time.Time `json:"created_at"`

	// Relations
	Question Question `json:"question" gorm:"foreignKey:QuestionID"`
}

// ===== QUESTION CONTENT SCHEMAS =====

type MultipleChoiceContent struct {
	Options          []MCOption `json:"options" validate:"min=2,max=10"`
	CorrectAnswers   []string   `json:"correct_answers" validate:"min=1"`
	MultipleCorrect  bool       `json:"multiple_correct"`
	RandomizeOptions bool       `json:"randomize_options"`
	PartialCredit    bool       `json:"partial_credit"`
}

type MCOption struct {
	ID       string  `json:"id"`
	Text     string  `json:"text" validate:"required"`
	ImageURL *string `json:"image_url"`
	Order    int     `json:"order"`
}

type TrueFalseContent struct {
	CorrectAnswer bool    `json:"correct_answer"`
	TrueLabel     *string `json:"true_label"` // Custom labels
	FalseLabel    *string `json:"false_label"`
}

type EssayContent struct {
	MinWords        *int     `json:"min_words"`
	MaxWords        *int     `json:"max_words"`
	SuggestedLength string   `json:"suggested_length"` // "2-3 paragraphs"
	RubricCriteria  []string `json:"rubric_criteria"`
	SampleAnswer    *string  `json:"sample_answer"`
	AutoGrade       bool     `json:"auto_grade"`
	KeyWords        []string `json:"key_words"` // For auto-grading
}

type FillBlankContent struct {
	Template      string              `json:"template"` // "The capital of {blank1} is {blank2}"
	Blanks        map[string]BlankDef `json:"blanks"`
	CaseSensitive bool                `json:"case_sensitive"`
	TrimSpaces    bool                `json:"trim_spaces"`
}

type BlankDef struct {
	AcceptedAnswers []string `json:"accepted_answers"`
	Points          int      `json:"points"`
	PlaceholderText *string  `json:"placeholder_text"`
}

type MatchingContent struct {
	LeftItems      []MatchItem `json:"left_items" validate:"min=2,max=10"`
	RightItems     []MatchItem `json:"right_items" validate:"min=2,max=10"`
	CorrectPairs   []MatchPair `json:"correct_pairs"`
	RandomizeLeft  bool        `json:"randomize_left"`
	RandomizeRight bool        `json:"randomize_right"`
	PartialCredit  bool        `json:"partial_credit"`
}

type MatchItem struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	ImageURL *string `json:"image_url"`
}

type MatchPair struct {
	LeftID  string `json:"left_id"`
	RightID string `json:"right_id"`
}

type OrderingContent struct {
	Items         []OrderItem `json:"items" validate:"min=2,max=10"`
	CorrectOrder  []string    `json:"correct_order"`
	RandomizeInit bool        `json:"randomize_initial"`
	PartialCredit bool        `json:"partial_credit"`
}

type OrderItem struct {
	ID       string  `json:"id"`
	Text     string  `json:"text"`
	ImageURL *string `json:"image_url"`
}

type ShortAnswerContent struct {
	AcceptedAnswers []string `json:"accepted_answers"`
	CaseSensitive   bool     `json:"case_sensitive"`
	ExactMatch      bool     `json:"exact_match"`
	MaxLength       int      `json:"max_length" validate:"min=1,max=500"`
	PlaceholderText *string  `json:"placeholder_text"`
	FuzzyMatching   bool     `json:"fuzzy_matching"`
}
