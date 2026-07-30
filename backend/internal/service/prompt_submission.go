package service

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	infraerrors "ikik-api/internal/pkg/errors"
)

const (
	PromptSubmissionStatusPending  = "pending"
	PromptSubmissionStatusApproved = "approved"
	PromptSubmissionStatusRejected = "rejected"

	maxPendingPromptSubmissions = 5
)

var (
	ErrPromptSubmissionNotFound = infraerrors.NotFound("PROMPT_SUBMISSION_NOT_FOUND", "prompt submission not found")
	ErrPromptSubmissionInvalid  = infraerrors.BadRequest("PROMPT_SUBMISSION_INVALID", "prompt submission is invalid")
	ErrPromptSubmissionLimit    = infraerrors.BadRequest("PROMPT_SUBMISSION_LIMIT", "too many pending prompt submissions")
)

type PromptSubmission struct {
	ID          int64      `json:"id"`
	UserID      int64      `json:"user_id"`
	Username    string     `json:"username"`
	UserEmail   string     `json:"user_email,omitempty"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Content     string     `json:"content"`
	Type        string     `json:"type"`
	Category    string     `json:"category"`
	MediaURL    string     `json:"media_url,omitempty"`
	Status      string     `json:"status"`
	ReviewNote  string     `json:"review_note,omitempty"`
	ReviewedBy  *int64     `json:"reviewed_by,omitempty"`
	ReviewedAt  *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type CreatePromptSubmissionInput struct {
	UserID      int64
	Title       string
	Description string
	Content     string
	Type        string
	Category    string
	MediaURL    string
}

type PromptSubmissionAdminFilters struct {
	Page     int
	PageSize int
	Status   string
	Search   string
}

type PromptSubmissionRepository interface {
	CreatePromptSubmission(ctx context.Context, input CreatePromptSubmissionInput) (*PromptSubmission, error)
	CountPendingPromptSubmissions(ctx context.Context, userID int64) (int, error)
	ListApprovedPromptSubmissions(ctx context.Context, page, pageSize int) ([]PromptSubmission, int64, error)
	ListPromptSubmissionsAdmin(ctx context.Context, filters PromptSubmissionAdminFilters) ([]PromptSubmission, int64, error)
	ReviewPromptSubmission(ctx context.Context, id, reviewerID int64, status, note string) (*PromptSubmission, error)
}

type PromptSubmissionService struct {
	repo PromptSubmissionRepository
}

func NewPromptSubmissionService(repo PromptSubmissionRepository) *PromptSubmissionService {
	return &PromptSubmissionService{repo: repo}
}

func (s *PromptSubmissionService) Submit(ctx context.Context, input CreatePromptSubmissionInput) (*PromptSubmission, error) {
	if s == nil || s.repo == nil {
		return nil, infraerrors.InternalServer("PROMPT_SUBMISSION_UNAVAILABLE", "prompt submission service is unavailable")
	}
	if input.UserID <= 0 {
		return nil, fmt.Errorf("%w: user is required", ErrPromptSubmissionInvalid)
	}
	input.Title = strings.TrimSpace(input.Title)
	input.Description = strings.TrimSpace(input.Description)
	input.Content = strings.TrimSpace(input.Content)
	input.Type = strings.ToUpper(strings.TrimSpace(input.Type))
	input.Category = strings.ToLower(strings.TrimSpace(input.Category))
	input.MediaURL = strings.TrimSpace(input.MediaURL)

	if len([]rune(input.Title)) == 0 || len([]rune(input.Title)) > 200 {
		return nil, fmt.Errorf("%w: title must be between 1 and 200 characters", ErrPromptSubmissionInvalid)
	}
	if len([]rune(input.Description)) > 500 {
		return nil, fmt.Errorf("%w: description must not exceed 500 characters", ErrPromptSubmissionInvalid)
	}
	if len([]rune(input.Content)) < 10 || len([]rune(input.Content)) > 20000 {
		return nil, fmt.Errorf("%w: content must be between 10 and 20000 characters", ErrPromptSubmissionInvalid)
	}
	if !validPromptSubmissionType(input.Type) {
		return nil, fmt.Errorf("%w: unsupported prompt type", ErrPromptSubmissionInvalid)
	}
	if !validPromptSubmissionCategory(input.Category) {
		input.Category = "other"
	}
	if input.MediaURL != "" && !validPromptSubmissionMediaURL(input.MediaURL) {
		return nil, fmt.Errorf("%w: media URL must use HTTPS", ErrPromptSubmissionInvalid)
	}

	pending, err := s.repo.CountPendingPromptSubmissions(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("count pending prompt submissions: %w", err)
	}
	if pending >= maxPendingPromptSubmissions {
		return nil, ErrPromptSubmissionLimit
	}
	return s.repo.CreatePromptSubmission(ctx, input)
}

func (s *PromptSubmissionService) ListApproved(ctx context.Context, page, pageSize int) ([]PromptSubmission, int64, error) {
	page, pageSize = normalizePromptSubmissionPagination(page, pageSize)
	return s.repo.ListApprovedPromptSubmissions(ctx, page, pageSize)
}

func (s *PromptSubmissionService) AdminList(ctx context.Context, filters PromptSubmissionAdminFilters) ([]PromptSubmission, int64, error) {
	filters.Page, filters.PageSize = normalizePromptSubmissionPagination(filters.Page, filters.PageSize)
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	filters.Search = strings.TrimSpace(filters.Search)
	if filters.Status != "" && filters.Status != "all" && !validPromptSubmissionStatus(filters.Status) {
		return nil, 0, fmt.Errorf("%w: unsupported status", ErrPromptSubmissionInvalid)
	}
	if filters.Status == "all" {
		filters.Status = ""
	}
	return s.repo.ListPromptSubmissionsAdmin(ctx, filters)
}

func (s *PromptSubmissionService) Review(ctx context.Context, id, reviewerID int64, status, note string) (*PromptSubmission, error) {
	status = strings.ToLower(strings.TrimSpace(status))
	note = strings.TrimSpace(note)
	if id <= 0 || reviewerID <= 0 || (status != PromptSubmissionStatusApproved && status != PromptSubmissionStatusRejected) {
		return nil, ErrPromptSubmissionInvalid
	}
	if len([]rune(note)) > 500 {
		return nil, fmt.Errorf("%w: review note must not exceed 500 characters", ErrPromptSubmissionInvalid)
	}
	return s.repo.ReviewPromptSubmission(ctx, id, reviewerID, status, note)
}

func validPromptSubmissionType(value string) bool {
	switch value {
	case "TEXT", "STRUCTURED", "IMAGE", "VIDEO", "AUDIO":
		return true
	default:
		return false
	}
}

func validPromptSubmissionCategory(value string) bool {
	switch value {
	case "coding", "writing", "business", "creative", "education", "workflow", "productivity", "other":
		return true
	default:
		return false
	}
}

func validPromptSubmissionStatus(value string) bool {
	return value == PromptSubmissionStatusPending || value == PromptSubmissionStatusApproved || value == PromptSubmissionStatusRejected
}

func validPromptSubmissionMediaURL(value string) bool {
	if len(value) > 2000 {
		return false
	}
	parsed, err := url.ParseRequestURI(value)
	return err == nil && parsed.Scheme == "https" && parsed.Host != ""
}

func normalizePromptSubmissionPagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
