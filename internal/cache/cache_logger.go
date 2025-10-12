package cache

import (
	"context"
	"fmt"
	"log/slog"
)

// SafeInvalidatePattern safely invalidates cache pattern with logging
func SafeInvalidatePattern(ctx context.Context, helper *CacheHelper, pattern string) {
	if err := helper.InvalidatePattern(ctx, pattern); err != nil {
		slog.ErrorContext(ctx, "Failed to invalidate cache pattern",
			"error", err,
			"pattern", pattern)
	}
}

// SafeDelete safely deletes cache keys with logging
func SafeDelete(ctx context.Context, helper *CacheHelper, keys ...string) {
	if err := helper.Delete(ctx, keys...); err != nil {
		slog.ErrorContext(ctx, "Failed to delete cache keys",
			"error", err,
			"keys", keys)
	}
}

// BatchInvalidate invalidates multiple patterns in batch
func BatchInvalidate(ctx context.Context, helper *CacheHelper, patterns []string) error {
	var lastErr error
	for _, pattern := range patterns {
		if err := helper.InvalidatePattern(ctx, pattern); err != nil {
			lastErr = err
			slog.ErrorContext(ctx, "Failed to invalidate pattern in batch",
				"error", err,
				"pattern", pattern)
		}
	}
	return lastErr
}

// InvalidateAssessmentCache invalidates all assessment-related caches using pipeline
func InvalidateAssessmentCache(ctx context.Context, cm *CacheManager, assessmentID uint, creatorID string) {
	// Delete specific keys using single call
	SafeDelete(ctx, cm.Assessment,
		fmt.Sprintf("id:%d", assessmentID),
		fmt.Sprintf("details:%d", assessmentID))

	// Invalidate patterns
	SafeInvalidatePattern(ctx, cm.Assessment, fmt.Sprintf("creator:%s:*", creatorID))
	SafeInvalidatePattern(ctx, cm.Assessment, "list:*")
	SafeInvalidatePattern(ctx, cm.Stats, fmt.Sprintf("assessment:%d:*", assessmentID))
}

// InvalidateQuestionCache invalidates all question-related caches
func InvalidateQuestionCache(ctx context.Context, cm *CacheManager, questionID uint, creatorID string) {
	SafeDelete(ctx, cm.Question, fmt.Sprintf("id:%d", questionID))
	SafeInvalidatePattern(ctx, cm.Question, fmt.Sprintf("creator:%s:*", creatorID))
	SafeInvalidatePattern(ctx, cm.Question, "list:*")
	SafeInvalidatePattern(ctx, cm.Stats, fmt.Sprintf("question:%d:*", questionID))
}
