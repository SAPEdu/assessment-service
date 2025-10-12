package postgres

import (
	"context"
	"fmt"

	"github.com/SAP-F-2025/assessment-service/internal/models"
	"github.com/SAP-F-2025/assessment-service/internal/repositories"
	"gorm.io/gorm"
)

type questionBankRepository struct {
	db *gorm.DB
}

func NewQuestionBankRepository(db *gorm.DB) repositories.QuestionBankRepository {
	return &questionBankRepository{db: db}
}

// ===== BASIC CRUD OPERATIONS =====

func (r *questionBankRepository) Create(ctx context.Context, tx *gorm.DB, bank *models.QuestionBank) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).Create(bank).Error; err != nil {
		return r.handleDBError(err, "create question bank")
	}
	return nil
}

func (r *questionBankRepository) GetByID(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionBank, error) {
	db := r.getDB(tx)
	var bank models.QuestionBank

	if err := db.WithContext(ctx).
		Preload("Creator").
		First(&bank, id).Error; err != nil {
		return nil, r.handleDBError(err, "get question bank by id")
	}

	return &bank, nil
}

func (r *questionBankRepository) GetByIDWithDetails(ctx context.Context, tx *gorm.DB, id uint) (*models.QuestionBank, error) {
	db := r.getDB(tx)
	var bank models.QuestionBank

	if err := db.WithContext(ctx).
		Preload("Creator").
		Preload("Questions").
		Preload("SharedWith.User").
		First(&bank, id).Error; err != nil {
		return nil, r.handleDBError(err, "get question bank with details")
	}

	return &bank, nil
}

func (r *questionBankRepository) Update(ctx context.Context, tx *gorm.DB, bank *models.QuestionBank) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).Save(bank).Error; err != nil {
		return r.handleDBError(err, "update question bank")
	}
	return nil
}

func (r *questionBankRepository) Delete(ctx context.Context, tx *gorm.DB, id uint) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).Delete(&models.QuestionBank{}, id).Error; err != nil {
		return r.handleDBError(err, "delete question bank")
	}
	return nil
}

// ===== QUERY OPERATIONS =====

func (r *questionBankRepository) List(ctx context.Context, tx *gorm.DB, filters repositories.QuestionBankFilters) ([]*models.QuestionBank, int64, error) {
	db := r.getDB(tx)
	var banks []*models.QuestionBank
	var total int64

	query := db.WithContext(ctx).Model(&models.QuestionBank{}).Preload("Creator")

	// Apply filters
	query = r.applyBankFilters(query, filters)

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, r.handleDBError(err, "count question banks")
	}

	// Apply pagination and sorting
	query = r.applyPaginationAndSorting(query, filters.Limit, filters.Offset, filters.SortBy, filters.SortOrder)

	if err := query.Find(&banks).Error; err != nil {
		return nil, 0, r.handleDBError(err, "list question banks")
	}

	return banks, total, nil
}

func (r *questionBankRepository) GetByCreator(ctx context.Context, tx *gorm.DB, creatorID string, filters repositories.QuestionBankFilters) ([]*models.QuestionBank, int64, error) {
	filters.CreatedBy = &creatorID
	return r.List(ctx, tx, filters)
}

func (r *questionBankRepository) GetPublicBanks(ctx context.Context, tx *gorm.DB, filters repositories.QuestionBankFilters) ([]*models.QuestionBank, int64, error) {
	isPublic := true
	filters.IsPublic = &isPublic
	return r.List(ctx, tx, filters)
}

func (r *questionBankRepository) GetSharedWithUser(ctx context.Context, tx *gorm.DB, userID string, filters repositories.QuestionBankFilters) ([]*models.QuestionBank, int64, error) {
	db := r.getDB(tx)
	var banks []*models.QuestionBank
	var total int64

	query := db.WithContext(ctx).
		Table("question_banks qb").
		Select("qb.*").
		Joins("INNER JOIN question_bank_shares qbs ON qb.id = qbs.bank_id").
		Where("qbs.user_id = ?", userID).
		Preload("Creator")

	// Apply filters
	if filters.Name != nil {
		query = query.Where("qb.name ILIKE ?", "%"+*filters.Name+"%")
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, r.handleDBError(err, "count shared question banks")
	}

	// Apply pagination and sorting
	query = r.applyPaginationAndSorting(query, filters.Limit, filters.Offset, filters.SortBy, filters.SortOrder)

	if err := query.Find(&banks).Error; err != nil {
		return nil, 0, r.handleDBError(err, "get shared question banks")
	}

	return banks, total, nil
}

func (r *questionBankRepository) Search(ctx context.Context, tx *gorm.DB, query string, filters repositories.QuestionBankFilters) ([]*models.QuestionBank, int64, error) {
	searchQuery := "%" + query + "%"

	db := r.getDB(tx)
	var banks []*models.QuestionBank
	var total int64

	dbQuery := db.WithContext(ctx).Model(&models.QuestionBank{}).
		Where("name ILIKE ? OR description ILIKE ?", searchQuery, searchQuery).
		Preload("Creator")

	// Apply filters
	dbQuery = r.applyBankFilters(dbQuery, filters)

	// Count total
	if err := dbQuery.Count(&total).Error; err != nil {
		return nil, 0, r.handleDBError(err, "count search results")
	}

	// Apply pagination and sorting
	dbQuery = r.applyPaginationAndSorting(dbQuery, filters.Limit, filters.Offset, filters.SortBy, filters.SortOrder)

	if err := dbQuery.Find(&banks).Error; err != nil {
		return nil, 0, r.handleDBError(err, "search question banks")
	}

	return banks, total, nil
}

// ===== SHARING OPERATIONS =====

func (r *questionBankRepository) ShareBank(ctx context.Context, tx *gorm.DB, share *models.QuestionBankShare) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).Create(share).Error; err != nil {
		return r.handleDBError(err, "share question bank")
	}
	return nil
}

func (r *questionBankRepository) UnshareBank(ctx context.Context, tx *gorm.DB, bankID uint, userID string) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).
		Where("bank_id = ? AND user_id = ?", bankID, userID).
		Delete(&models.QuestionBankShare{}).Error; err != nil {
		return r.handleDBError(err, "unshare question bank")
	}
	return nil
}

func (r *questionBankRepository) UpdateSharePermissions(ctx context.Context, tx *gorm.DB, bankID uint, userID string, canEdit, canDelete bool) error {
	db := r.getDB(tx)
	if err := db.WithContext(ctx).
		Model(&models.QuestionBankShare{}).
		Where("bank_id = ? AND user_id = ?", bankID, userID).
		Updates(map[string]interface{}{
			"can_edit":   canEdit,
			"can_delete": canDelete,
		}).Error; err != nil {
		return r.handleDBError(err, "update share permissions")
	}
	return nil
}

func (r *questionBankRepository) GetBankShares(ctx context.Context, tx *gorm.DB, bankID uint) ([]*models.QuestionBankShare, error) {
	db := r.getDB(tx)
	var shares []*models.QuestionBankShare

	if err := db.WithContext(ctx).
		Where("bank_id = ?", bankID).
		Preload("User").
		Preload("Sharer").
		Find(&shares).Error; err != nil {
		return nil, r.handleDBError(err, "get bank shares")
	}

	return shares, nil
}

func (r *questionBankRepository) GetUserShares(ctx context.Context, tx *gorm.DB, userID string, filters repositories.QuestionBankShareFilters) ([]*models.QuestionBankShare, int64, error) {
	db := r.getDB(tx)
	var shares []*models.QuestionBankShare
	var total int64

	query := db.WithContext(ctx).Model(&models.QuestionBankShare{}).
		Where("user_id = ?", userID).
		Preload("Bank").
		Preload("Sharer")

	// Apply filters
	if filters.BankID != nil {
		query = query.Where("bank_id = ?", *filters.BankID)
	}
	if filters.CanEdit != nil {
		query = query.Where("can_edit = ?", *filters.CanEdit)
	}
	if filters.CanDelete != nil {
		query = query.Where("can_delete = ?", *filters.CanDelete)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, r.handleDBError(err, "count user shares")
	}

	// Apply pagination
	if filters.Limit > 0 {
		query = query.Limit(filters.Limit)
	}
	if filters.Offset > 0 {
		query = query.Offset(filters.Offset)
	}

	if err := query.Find(&shares).Error; err != nil {
		return nil, 0, r.handleDBError(err, "get user shares")
	}

	return shares, total, nil
}

// ===== QUESTION-BANK RELATIONSHIP OPERATIONS =====

func (r *questionBankRepository) AddQuestions(ctx context.Context, tx *gorm.DB, bankID uint, questionIDs []uint) error {
	db := r.getDB(tx)

	// Get the bank first
	var bank models.QuestionBank
	if err := db.WithContext(ctx).First(&bank, bankID).Error; err != nil {
		return r.handleDBError(err, "get bank for adding questions")
	}

	// Get questions to add
	var questions []models.Question
	if err := db.WithContext(ctx).Find(&questions, questionIDs).Error; err != nil {
		return r.handleDBError(err, "get questions to add to bank")
	}

	// Add questions to bank
	if err := db.WithContext(ctx).Model(&bank).Association("Questions").Append(&questions); err != nil {
		return r.handleDBError(err, "add questions to bank")
	}

	return nil
}

func (r *questionBankRepository) RemoveQuestions(ctx context.Context, tx *gorm.DB, bankID uint, questionIDs []uint) error {
	db := r.getDB(tx)

	// Get the bank first
	var bank models.QuestionBank
	if err := db.WithContext(ctx).First(&bank, bankID).Error; err != nil {
		return r.handleDBError(err, "get bank for removing questions")
	}

	// Get questions to remove
	var questions []models.Question
	if err := db.WithContext(ctx).Find(&questions, questionIDs).Error; err != nil {
		return r.handleDBError(err, "get questions to remove from bank")
	}

	// Remove questions from bank
	if err := db.WithContext(ctx).Model(&bank).Association("Questions").Delete(&questions); err != nil {
		return r.handleDBError(err, "remove questions from bank")
	}

	return nil
}

func (r *questionBankRepository) GetBankQuestions(ctx context.Context, tx *gorm.DB, bankID uint, filters repositories.QuestionFilters) ([]*models.Question, int64, error) {
	db := r.getDB(tx)
	var questions []*models.Question
	var total int64

	query := db.WithContext(ctx).
		Table("questions q").
		Joins("INNER JOIN question_bank_questions qbq ON q.id = qbq.question_id").
		Where("qbq.question_bank_id = ?", bankID).
		Preload("Category").
		Preload("Creator")

	// Apply question filters
	if filters.Type != nil {
		query = query.Where("q.type = ?", *filters.Type)
	}
	if filters.Difficulty != nil {
		query = query.Where("q.difficulty = ?", *filters.Difficulty)
	}
	if filters.CategoryID != nil {
		query = query.Where("q.category_id = ?", *filters.CategoryID)
	}

	// Count total
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, r.handleDBError(err, "count bank questions")
	}

	// Apply pagination and sorting
	query = r.applyQuestionPaginationAndSorting(query, filters.Limit, filters.Offset, filters.SortBy, filters.SortOrder)

	if err := query.Select("q.*").Find(&questions).Error; err != nil {
		return nil, 0, r.handleDBError(err, "get bank questions")
	}

	return questions, total, nil
}

func (r *questionBankRepository) IsQuestionInBank(ctx context.Context, tx *gorm.DB, questionID, bankID uint) (bool, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Table("question_bank_questions").
		Where("question_id = ? AND question_bank_id = ?", questionID, bankID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check if question in bank")
	}

	return count > 0, nil
}

// ===== PERMISSION CHECKS =====

func (r *questionBankRepository) CanAccess(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error) {
	db := r.getDB(tx)

	// Check if user is owner
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("id = ? AND created_by = ?", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check bank ownership")
	}
	if count > 0 {
		return true, nil
	}

	// Check if bank is public
	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("id = ? AND is_public = true", bankID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check if bank is public")
	}
	if count > 0 {
		return true, nil
	}

	// Check if shared with user
	if err := db.WithContext(ctx).
		Model(&models.QuestionBankShare{}).
		Where("bank_id = ? AND user_id = ?", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check if bank is shared with user")
	}

	return count > 0, nil
}

func (r *questionBankRepository) CanEdit(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error) {
	db := r.getDB(tx)

	// Check if user is owner
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("id = ? AND created_by = ?", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check bank ownership for edit")
	}
	if count > 0 {
		return true, nil
	}

	// Check if user has edit permission through sharing
	if err := db.WithContext(ctx).
		Model(&models.QuestionBankShare{}).
		Where("bank_id = ? AND user_id = ? AND can_edit = true", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check edit permission through sharing")
	}

	return count > 0, nil
}

func (r *questionBankRepository) CanDelete(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error) {
	db := r.getDB(tx)

	// Check if user is owner
	var count int64
	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("id = ? AND created_by = ?", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check bank ownership for delete")
	}
	if count > 0 {
		return true, nil
	}

	// Check if user has delete permission through sharing
	if err := db.WithContext(ctx).
		Model(&models.QuestionBankShare{}).
		Where("bank_id = ? AND user_id = ? AND can_delete = true", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check delete permission through sharing")
	}

	return count > 0, nil
}

func (r *questionBankRepository) IsOwner(ctx context.Context, tx *gorm.DB, bankID uint, userID string) (bool, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("id = ? AND created_by = ?", bankID, userID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check bank ownership")
	}

	return count > 0, nil
}

// ===== VALIDATION =====

func (r *questionBankRepository) ExistsByName(ctx context.Context, tx *gorm.DB, name string, creatorID string) (bool, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Model(&models.QuestionBank{}).
		Where("name = ? AND created_by = ?", name, creatorID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check if bank name exists")
	}

	return count > 0, nil
}

func (r *questionBankRepository) HasQuestions(ctx context.Context, tx *gorm.DB, bankID uint) (bool, error) {
	db := r.getDB(tx)
	var count int64

	if err := db.WithContext(ctx).
		Table("question_bank_questions").
		Where("question_bank_id = ?", bankID).
		Count(&count).Error; err != nil {
		return false, r.handleDBError(err, "check if bank has questions")
	}

	return count > 0, nil
}

// ===== STATISTICS =====

func (r *questionBankRepository) GetBankStats(ctx context.Context, tx *gorm.DB, bankID uint) (*repositories.QuestionBankStats, error) {
	db := r.getDB(tx)
	stats := &repositories.QuestionBankStats{
		QuestionsByType: make(map[models.QuestionType]int),
		QuestionsByDiff: make(map[models.DifficultyLevel]int),
	}

	// Count total questions
	var questionCount int64
	if err := db.WithContext(ctx).
		Table("question_bank_questions").
		Where("question_bank_id = ?", bankID).
		Count(&questionCount).Error; err != nil {
		return nil, r.handleDBError(err, "count bank questions")
	}
	stats.QuestionCount = int(questionCount)

	// Count questions by type
	type TypeCount struct {
		Type  models.QuestionType `json:"type"`
		Count int                 `json:"count"`
	}
	var typeCounts []TypeCount
	if err := db.WithContext(ctx).
		Table("questions q").
		Select("q.type, COUNT(*) as count").
		Joins("INNER JOIN question_bank_questions qbq ON q.id = qbq.question_id").
		Where("qbq.question_bank_id = ?", bankID).
		Group("q.type").
		Scan(&typeCounts).Error; err != nil {
		return nil, r.handleDBError(err, "count questions by type")
	}

	for _, tc := range typeCounts {
		stats.QuestionsByType[tc.Type] = tc.Count
	}

	// Count questions by difficulty
	type DiffCount struct {
		Difficulty models.DifficultyLevel `json:"difficulty"`
		Count      int                    `json:"count"`
	}
	var diffCounts []DiffCount
	if err := db.WithContext(ctx).
		Table("questions q").
		Select("q.difficulty, COUNT(*) as count").
		Joins("INNER JOIN question_bank_questions qbq ON q.id = qbq.question_id").
		Where("qbq.question_bank_id = ?", bankID).
		Group("q.difficulty").
		Scan(&diffCounts).Error; err != nil {
		return nil, r.handleDBError(err, "count questions by difficulty")
	}

	for _, dc := range diffCounts {
		stats.QuestionsByDiff[dc.Difficulty] = dc.Count
	}

	// Count shares
	var shareCount int64
	if err := db.WithContext(ctx).
		Model(&models.QuestionBankShare{}).
		Where("bank_id = ?", bankID).
		Count(&shareCount).Error; err != nil {
		return nil, r.handleDBError(err, "count bank shares")
	}
	stats.ShareCount = int(shareCount)

	// TODO: Add usage count and last used from assessment usage

	return stats, nil
}

func (r *questionBankRepository) GetUsageCount(ctx context.Context, tx *gorm.DB, bankID uint) (int, error) {
	// TODO: Implement based on assessment usage tracking
	// For now, return 0
	return 0, nil
}

func (r *questionBankRepository) UpdateUsage(ctx context.Context, tx *gorm.DB, bankID uint) error {
	// TODO: Implement usage tracking updates
	// This would typically be called when a bank is used in an assessment
	return nil
}

// ===== HELPER METHODS =====

func (r *questionBankRepository) getDB(tx *gorm.DB) *gorm.DB {
	if tx != nil {
		return tx
	}
	return r.db
}

func (r *questionBankRepository) handleDBError(err error, operation string) error {
	return handleDBError(err, operation)
}

// handleDBError is a package-level helper for handling database errors
func handleDBError(err error, operation string) error {
	if err == nil {
		return nil
	}

	// You can customize this based on your error handling strategy
	return fmt.Errorf("%s failed: %w", operation, err)
}

func (r *questionBankRepository) applyBankFilters(query *gorm.DB, filters repositories.QuestionBankFilters) *gorm.DB {
	if filters.IsPublic != nil {
		query = query.Where("is_public = ?", *filters.IsPublic)
	}
	if filters.IsShared != nil {
		query = query.Where("is_shared = ?", *filters.IsShared)
	}
	if filters.CreatedBy != nil {
		query = query.Where("created_by = ?", *filters.CreatedBy)
	}
	if filters.Name != nil {
		query = query.Where("name ILIKE ?", "%"+*filters.Name+"%")
	}

	return query
}

func (r *questionBankRepository) applyPaginationAndSorting(query *gorm.DB, limit, offset int, sortBy, sortOrder string) *gorm.DB {
	// Whitelist allowed sort columns: map API keys to SQL identifiers
	sortKeyToColumn := map[string]string{
		"created_at": "created_at",
		"updated_at": "updated_at",
		"name":       "name",
		"id":         "id",
	}

	// Validate and set sort column (map API to SQL name, default if invalid)
	column, ok := sortKeyToColumn[sortBy]
	if !ok {
		column = "created_at"
	}

	// Validate and set sort order
	order := "DESC"
	if sortOrder == "asc" || sortOrder == "ASC" {
		order = "ASC"
	}

	// Use only mapped SQL column name and constant sort order
	query = query.Order(fmt.Sprintf("%s %s", column, order))

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query
}

func (r *questionBankRepository) applyQuestionPaginationAndSorting(query *gorm.DB, limit, offset int, sortBy, sortOrder string) *gorm.DB {
	// Whitelist allowed sort columns for questions
	// Map logical API sort keys (must match handler) to SQL-safe column names
	sortKeyToColumn := map[string]string{
		"created_at": "q.created_at",
		"updated_at": "q.updated_at",
		"difficulty": "q.difficulty",
		"type":       "q.type",
	}

	column, ok := sortKeyToColumn[sortBy]
	if !ok {
		column = "q.created_at"
	}

	sortOrderUpper := "DESC"
	if sortOrder == "asc" || sortOrder == "ASC" {
		sortOrderUpper = "ASC"
	}

	// Only use safe mapped column and constant sort order
	query = query.Order(fmt.Sprintf("%s %s", column, sortOrderUpper))

	// Apply pagination
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}

	return query
}
