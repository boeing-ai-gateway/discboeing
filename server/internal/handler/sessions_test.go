package handler

import (
	"testing"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/service"
)

func TestMapSessionResponseIncludesThreadStatus(t *testing.T) {
	t.Parallel()

	response := mapSessionResponse(&service.Session{
		ID:     "session-1",
		Status: model.SessionStatusReady,
		ThreadStatus: &service.SessionActivityStatus{
			Status:                 model.SessionActivityStatusNeedsAttention,
			Reason:                 model.SessionActivityReasonPendingQuestion,
			NeedsAttentionCount:    1,
			RepresentativeThreadID: "thread-1",
		},
	})

	if response.ThreadStatus == nil {
		t.Fatal("expected thread status to be included")
	}
	if response.ThreadStatus.Status != model.SessionActivityStatusNeedsAttention {
		t.Fatalf("thread status = %q, want %q", response.ThreadStatus.Status, model.SessionActivityStatusNeedsAttention)
	}
}

func TestDeriveSessionStatusAndError_MapsCompletedCommitOperation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		session    *service.Session
		wantStatus string
	}{
		{
			name: "commit completion maps to committed",
			session: &service.Session{
				Status:          "ready",
				CommitStatus:    "completed",
				CommitOperation: service.CommitOperationCommit,
			},
			wantStatus: "committed",
		},
		{
			name: "unknown completed operation falls back to session status",
			session: &service.Session{
				Status:          "ready",
				CommitStatus:    "completed",
				CommitOperation: "rebase",
			},
			wantStatus: "ready",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotStatus, gotError := deriveSessionStatusAndError(tt.session)
			if gotStatus != tt.wantStatus {
				t.Fatalf("deriveSessionStatusAndError() status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotError != "" {
				t.Fatalf("deriveSessionStatusAndError() error = %q, want empty", gotError)
			}
		})
	}
}

func TestDeriveSessionStatusAndError_DoesNotMapCommitFailureToError(t *testing.T) {
	t.Parallel()

	gotStatus, gotError := deriveSessionStatusAndError(&service.Session{
		Status:          model.SessionStatusReady,
		CommitStatus:    model.CommitStatusFailed,
		CommitOperation: service.CommitOperationCommit,
		CommitError:     "commit failed",
	})
	if gotStatus != model.SessionStatusReady {
		t.Fatalf("deriveSessionStatusAndError() status = %q, want %q", gotStatus, model.SessionStatusReady)
	}
	if gotError != "" {
		t.Fatalf("deriveSessionStatusAndError() error = %q, want empty", gotError)
	}
}

func TestDeriveSessionStatusAndError_RemovingOverridesDerivedCommitStates(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		session    *service.Session
		wantStatus string
	}{
		{
			name: "removing overrides committed",
			session: &service.Session{
				Status:          model.SessionStatusRemoving,
				CommitStatus:    model.CommitStatusCompleted,
				CommitOperation: service.CommitOperationCommit,
			},
			wantStatus: model.SessionStatusRemoving,
		},
		{
			name: "removing overrides commit failure",
			session: &service.Session{
				Status:          model.SessionStatusRemoving,
				CommitStatus:    model.CommitStatusFailed,
				CommitOperation: service.CommitOperationCommit,
				CommitError:     "commit failed",
			},
			wantStatus: model.SessionStatusRemoving,
		},
		{
			name: "removed overrides completed unknown operation",
			session: &service.Session{
				Status:          model.SessionStatusRemoved,
				CommitStatus:    model.CommitStatusCompleted,
				CommitOperation: "rebase",
			},
			wantStatus: model.SessionStatusRemoved,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotStatus, gotError := deriveSessionStatusAndError(tt.session)
			if gotStatus != tt.wantStatus {
				t.Fatalf("deriveSessionStatusAndError() status = %q, want %q", gotStatus, tt.wantStatus)
			}
			if gotError != "" {
				t.Fatalf("deriveSessionStatusAndError() error = %q, want empty", gotError)
			}
		})
	}
}
