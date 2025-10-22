package services

import (
	"log/slog"
	"testing"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"github.com/SAP-F-2025/assessment-service/internal/validator"
	"gorm.io/gorm"
)

func TestNewGradingService(t *testing.T) {
	type args struct {
		db        *gorm.DB
		repo      repositories.Repository
		logger    *slog.Logger
		validator *validator.Validator
	}
	tests := []struct {
		name string
		args args
		want GradingService
	}{
		{name: "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewGradingService(tt.args.db, tt.args.repo, tt.args.logger, tt.args.validator)
		})
	}
}
