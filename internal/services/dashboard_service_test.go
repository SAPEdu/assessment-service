package services

import (
	"log/slog"
	"testing"

	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

func TestNewDashboardService(t *testing.T) {
	type args struct {
		repo   repositories.Repository
		db     *gorm.DB
		logger *slog.Logger
	}
	tests := []struct {
		name string
		args args
		want DashboardService
	}{
		{
			name: "ok",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NewDashboardService(tt.args.repo, tt.args.db, tt.args.logger)
		})
	}
}
