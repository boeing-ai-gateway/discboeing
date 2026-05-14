package handler

import (
	"testing"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/service"
)

func TestMapSessionResponseIncludesThreadStatus(t *testing.T) {
	t.Parallel()

	response := mapSessionResponse(&service.Session{
		ID:            "session-1",
		SandboxStatus: model.SessionStatusReady,
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

func TestMapSessionResponseKeepsLifecycleAndCommitStatusSeparate(t *testing.T) {
	t.Parallel()

	response := mapSessionResponse(&service.Session{
		ID:              "session-1",
		SandboxStatus:   model.SessionStatusReady,
		CommitStatus:    model.CommitStatusCompleted,
		CommitOperation: service.CommitOperationCommit,
	})

	if response.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("status = %q, want %q", response.SandboxStatus, model.SessionStatusReady)
	}
	if response.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("sandbox status = %q, want %q", response.SandboxStatus, model.SessionStatusReady)
	}
	if response.CommitStatus != model.CommitStatusCompleted {
		t.Fatalf("commit status = %q, want %q", response.CommitStatus, model.CommitStatusCompleted)
	}
	if response.CommitOperation != service.CommitOperationCommit {
		t.Fatalf("commit operation = %q, want %q", response.CommitOperation, service.CommitOperationCommit)
	}
}

func TestMapSessionResponsePreservesRemovingStatusWithCommitFields(t *testing.T) {
	t.Parallel()

	response := mapSessionResponse(&service.Session{
		ID:              "session-1",
		SandboxStatus:   model.SessionStatusRemoving,
		CommitStatus:    model.CommitStatusCompleted,
		CommitOperation: service.CommitOperationCommit,
	})

	if response.SandboxStatus != model.SessionStatusRemoving {
		t.Fatalf("status = %q, want %q", response.SandboxStatus, model.SessionStatusRemoving)
	}
	if response.CommitStatus != model.CommitStatusCompleted {
		t.Fatalf("commit status = %q, want %q", response.CommitStatus, model.CommitStatusCompleted)
	}
}

func TestMapSessionResponseDoesNotMapCommitFailureToSessionError(t *testing.T) {
	t.Parallel()

	response := mapSessionResponse(&service.Session{
		ID:              "session-1",
		SandboxStatus:   model.SessionStatusReady,
		CommitStatus:    model.CommitStatusFailed,
		CommitOperation: service.CommitOperationCommit,
		CommitError:     "commit failed",
	})

	if response.SandboxStatus != model.SessionStatusReady {
		t.Fatalf("status = %q, want %q", response.SandboxStatus, model.SessionStatusReady)
	}
	if response.ErrorMessage != "" {
		t.Fatalf("error message = %q, want empty", response.ErrorMessage)
	}
	if response.CommitError != "commit failed" {
		t.Fatalf("commit error = %q, want commit failed", response.CommitError)
	}
}
