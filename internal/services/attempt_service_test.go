package services

import (
	"log/slog"
	"testing"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

func TestNewAttemptService(t *testing.T) {
	type args struct {
		repo      repositories.Repository
		db        *gorm.DB
		logger    *slog.Logger
		validator *validator.Validator
	}
	tests := []struct {
		name string
		args args
		want AttemptService
	}{
		{name: "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewAttemptService(tt.args.repo, tt.args.db, tt.args.logger, tt.args.validator, nil)
		})
	}
}
