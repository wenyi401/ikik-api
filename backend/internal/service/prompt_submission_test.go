package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

type promptSubmissionRepoStub struct {
	pending int
	created CreatePromptSubmissionInput
}

func (r *promptSubmissionRepoStub) CreatePromptSubmission(_ context.Context, input CreatePromptSubmissionInput) (*PromptSubmission, error) {
	r.created = input
	return &PromptSubmission{ID: 1, UserID: input.UserID, Title: input.Title, Status: PromptSubmissionStatusPending}, nil
}
func (r *promptSubmissionRepoStub) CountPendingPromptSubmissions(context.Context, int64) (int, error) {
	return r.pending, nil
}
func (r *promptSubmissionRepoStub) ListApprovedPromptSubmissions(context.Context, int, int) ([]PromptSubmission, int64, error) {
	return nil, 0, nil
}
func (r *promptSubmissionRepoStub) ListPromptSubmissionsAdmin(context.Context, PromptSubmissionAdminFilters) ([]PromptSubmission, int64, error) {
	return nil, 0, nil
}
func (r *promptSubmissionRepoStub) ReviewPromptSubmission(context.Context, int64, int64, string, string) (*PromptSubmission, error) {
	return nil, nil
}

func TestPromptSubmissionServiceSubmitNormalizesInput(t *testing.T) {
	repo := &promptSubmissionRepoStub{}
	svc := NewPromptSubmissionService(repo)
	created, err := svc.Submit(context.Background(), CreatePromptSubmissionInput{
		UserID: 7, Title: "  Code review  ", Content: "  Review this code carefully.  ", Type: "text", Category: "coding",
	})
	require.NoError(t, err)
	require.Equal(t, int64(1), created.ID)
	require.Equal(t, "Code review", repo.created.Title)
	require.Equal(t, "TEXT", repo.created.Type)
	require.Equal(t, "coding", repo.created.Category)
}

func TestPromptSubmissionServiceSubmitRejectsUnsafeMediaURL(t *testing.T) {
	svc := NewPromptSubmissionService(&promptSubmissionRepoStub{})
	_, err := svc.Submit(context.Background(), CreatePromptSubmissionInput{
		UserID: 7, Title: "Image prompt", Content: "Generate a clean product image.", Type: "IMAGE", MediaURL: "javascript:alert(1)",
	})
	require.ErrorIs(t, err, ErrPromptSubmissionInvalid)
}

func TestPromptSubmissionServiceSubmitLimitsPendingItems(t *testing.T) {
	svc := NewPromptSubmissionService(&promptSubmissionRepoStub{pending: maxPendingPromptSubmissions})
	_, err := svc.Submit(context.Background(), CreatePromptSubmissionInput{
		UserID: 7, Title: "Useful prompt", Content: "Provide a detailed useful answer.", Type: "TEXT",
	})
	require.ErrorIs(t, err, ErrPromptSubmissionLimit)
}
