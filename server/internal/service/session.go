package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/obot-platform/discobot/server/internal/events"
	"github.com/obot-platform/discobot/server/internal/git"
	"github.com/obot-platform/discobot/server/internal/jobs"
	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/sandbox/sandboxapi"
	"github.com/obot-platform/discobot/server/internal/store"
)

// SessionIDMaxLength is the maximum allowed length for a session ID.
const SessionIDMaxLength = 65

// Commit operation constants for API/session state.
const (
	CommitOperationCommit = model.CommitOperationCommit
)

// ErrSessionOperationInProgress indicates a commit operation is already running.
var ErrSessionOperationInProgress = errors.New("session operation already in progress")

// CommitsNoOpError is returned by GetCommits when the sandbox has no commits beyond the
// base, carrying the sandbox's working-tree state so callers can decide whether the
// absence of commits is safe to treat as a completed no-op.
type CommitsNoOpError struct {
	IsClean    bool   // true when the sandbox working tree has no uncommitted changes
	HeadCommit string // the sandbox HEAD commit SHA
}

type CommitSessionOptions struct {
	RequestedDirectory  string
	RequestedBaseCommit string
	RequestedCommitHash string
	ApprovalThreadID    string
	ApprovalQuestionID  string
}

func (e *CommitsNoOpError) Error() string {
	return fmt.Sprintf("commits error (no_commits): head=%s isClean=%v", e.HeadCommit, e.IsClean)
}

const (
	legacySessionStatusRunning          = "running"
	defaultSandboxCleanupRetentionDelay = 1 * time.Minute
	defaultSessionTargetRef             = "HEAD"
	threadStatusRefreshTimeout          = 2 * time.Second
)

// sessionIDRegex matches valid session IDs (alphanumeric and hyphens only).
const (
	requestCommitPullSucceededKey = "__request_commit_pull_succeeded__"
	requestCommitPullFailedKey    = "__request_commit_pull_failed__"
	requestCommitPullResultKey    = "__request_commit_pull_result__"
)

var sessionIDRegex = regexp.MustCompile(`^[a-zA-Z0-9-]+$`)

// ValidateSessionID validates that a session ID meets format requirements:
// - Only alphanumeric characters (a-z, A-Z, 0-9) and hyphens (-) are allowed
// - Maximum length is 65 characters
func ValidateSessionID(sessionID string) error {
	if sessionID == "" {
		return errors.New("session ID is required")
	}
	if len(sessionID) > SessionIDMaxLength {
		return fmt.Errorf("session ID exceeds maximum length of %d characters", SessionIDMaxLength)
	}
	if !sessionIDRegex.MatchString(sessionID) {
		return errors.New("session ID must contain only alphanumeric characters and hyphens")
	}
	return nil
}

func normalizeSessionStatus(status string) string {
	if status == legacySessionStatusRunning {
		return model.SessionStatusReady
	}
	return status
}

// Session represents a chat session (for API responses)
type Session struct {
	ID              string                 `json:"id"`
	ProjectID       string                 `json:"projectId"`
	ProviderID      string                 `json:"providerId,omitempty"`
	Name            string                 `json:"name"`
	DisplayName     string                 `json:"displayName,omitempty"`
	Description     string                 `json:"description"`
	CreatedAt       string                 `json:"createdAt"`
	Timestamp       string                 `json:"timestamp"`
	Status          string                 `json:"status"`
	CommitStatus    string                 `json:"commitStatus,omitempty"`
	CommitOperation string                 `json:"commitOperation,omitempty"`
	CommitError     string                 `json:"commitError,omitempty"`
	TargetRef       string                 `json:"targetRef,omitempty"`
	AppliedCommit   string                 `json:"appliedCommit,omitempty"`
	ErrorMessage    string                 `json:"errorMessage,omitempty"`
	ThreadStatus    *SessionActivityStatus `json:"threadStatus,omitempty"`
	Files           []FileNode             `json:"files"`
	WorkspaceID     string                 `json:"workspaceId,omitempty"`
	WorkspacePath   string                 `json:"workspacePath,omitempty"`
}

// FileNode represents a file in a session
type FileNode struct {
	ID              string     `json:"id"`
	Name            string     `json:"name"`
	Type            string     `json:"type"`
	Children        []FileNode `json:"children,omitempty"`
	Content         string     `json:"content,omitempty"`
	OriginalContent string     `json:"originalContent,omitempty"`
	Changed         bool       `json:"changed,omitempty"`
}

// SessionService handles session operations
type SessionService struct {
	store          *store.Store
	gitService     *GitService
	sandboxService *SandboxService
	eventBroker    *events.Broker
	jobEnqueuer    JobEnqueuer
	cleanupDelay   time.Duration
}

// NewSessionService creates a new session service
func NewSessionService(s *store.Store, gitSvc *GitService, sandboxService *SandboxService, eventBroker *events.Broker, jobEnqueuer JobEnqueuer) *SessionService {
	return &SessionService{
		store:          s,
		gitService:     gitSvc,
		sandboxService: sandboxService,
		eventBroker:    eventBroker,
		jobEnqueuer:    jobEnqueuer,
		cleanupDelay:   defaultSandboxCleanupRetentionDelay,
	}
}

// SetSandboxCleanupDelay updates the retention window used before permanently
// removing a sandbox after the session has been deleted.
func (s *SessionService) SetSandboxCleanupDelay(delay time.Duration) {
	if delay < 0 {
		delay = 0
	}
	s.cleanupDelay = delay
}

// ListSessionsByProject returns all sessions for a project.
func (s *SessionService) ListSessionsByProject(ctx context.Context, projectID string) ([]*Session, error) {
	dbSessions, err := s.store.ListSessionsByProject(ctx, projectID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*Session, len(dbSessions))
	for i, sess := range dbSessions {
		sessions[i] = s.mapSession(sess)
	}
	return sessions, nil
}

// ListSessionsByWorkspace returns all sessions for a workspace.
// Deprecated: prefer ListSessionsByProject for project-scoped session listing.
func (s *SessionService) ListSessionsByWorkspace(ctx context.Context, workspaceID string) ([]*Session, error) {
	dbSessions, err := s.store.ListSessionsByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("failed to list sessions: %w", err)
	}

	sessions := make([]*Session, len(dbSessions))
	for i, sess := range dbSessions {
		sessions[i] = s.mapSession(sess)
	}
	return sessions, nil
}

// GetSession returns a session by ID
func (s *SessionService) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	s.syncSessionNameFromPrimaryThread(ctx, sess)

	return s.mapSession(sess), nil
}

func (s *SessionService) syncSessionNameFromPrimaryThread(ctx context.Context, sess *model.Session) {
	if sess == nil || strings.TrimSpace(sess.Name) != "" || s.sandboxService == nil {
		return
	}
	if normalizeSessionStatus(sess.Status) != model.SessionStatusReady {
		return
	}

	client, err := s.sandboxService.GetClient(ctx, sess.ID)
	if err != nil {
		return
	}

	thread, err := client.GetThread(ctx, sess.ID)
	if err != nil {
		return
	}

	name := strings.TrimSpace(thread.Name)
	if name == "" {
		return
	}

	if _, err := s.UpdateSession(ctx, sess.ID, name, nil, ""); err != nil {
		log.Printf("Warning: failed to sync session name from thread for %s: %v", sess.ID, err)
		return
	}

	sess.Name = name
}

// CreateSession creates a new session with initializing status and auto-generated ID.
// If initialMessage is provided, it creates the first user message in the session.
func (s *SessionService) CreateSession(ctx context.Context, projectID, workspaceID, name, initialMessage string) (*Session, error) {
	return s.CreateSessionWithProvider(ctx, projectID, workspaceID, "", name, initialMessage)
}

// CreateSessionWithProvider creates a new session with a selected sandbox provider instance.
func (s *SessionService) CreateSessionWithProvider(ctx context.Context, projectID, workspaceID, providerID, name, initialMessage string) (*Session, error) {
	sess := &model.Session{
		ProjectID:         projectID,
		WorkspaceID:       workspaceID,
		SandboxProviderID: strings.TrimSpace(providerID),
		Name:              name,
		Description:       nil,
		Status:            model.SessionStatusInitializing,
		ThreadStatus:      model.SessionActivityStatusIdle,
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	// Create the initial user message if provided
	if initialMessage != "" {
		msg := &model.Message{
			SessionID: sess.ID,
			Role:      "user",
			Parts:     model.NewTextParts(initialMessage),
		}
		if err := s.store.CreateMessage(ctx, msg); err != nil {
			// Log the error but don't fail session creation
			log.Printf("Warning: failed to create initial message for session %s: %v", sess.ID, err)
		}
	}

	return s.mapSession(sess), nil
}

// CreateSessionWithID creates a new session with the provided client ID.
func (s *SessionService) CreateSessionWithID(ctx context.Context, sessionID, projectID, workspaceID, name string) (*Session, error) {
	return s.CreateSessionWithIDAndProvider(ctx, sessionID, projectID, workspaceID, "", name)
}

// CreateSessionWithIDAndProvider creates a new session with the provided client ID and sandbox provider instance.
func (s *SessionService) CreateSessionWithIDAndProvider(ctx context.Context, sessionID, projectID, workspaceID, providerID, name string) (*Session, error) {
	sess := &model.Session{
		ID:                sessionID, // Use client-provided ID
		ProjectID:         projectID,
		WorkspaceID:       workspaceID,
		SandboxProviderID: strings.TrimSpace(providerID),
		Name:              name,
		Description:       nil,
		Status:            model.SessionStatusInitializing,
		ThreadStatus:      model.SessionActivityStatusIdle,
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}

	return s.mapSession(sess), nil
}

// UpdateStatus updates the session status and optional error message, and publishes an SSE event.
func (s *SessionService) UpdateStatus(ctx context.Context, projectID, sessionID, status string, errorMsg *string) (*Session, error) {
	status = normalizeSessionStatus(status)

	// Use targeted column update to avoid overwriting concurrent changes to other fields
	if err := s.store.UpdateSessionStatus(ctx, sessionID, status, errorMsg); err != nil {
		return nil, fmt.Errorf("failed to update session status: %w", err)
	}

	// Re-read the session to return the full updated state
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	// Always publish SSE event for status changes
	if s.eventBroker != nil {
		if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, status, ""); err != nil {
			log.Printf("Failed to publish session update event: %v", err)
		}
	}

	return s.mapSession(sess), nil
}

// UpdateErrorMessage updates only the session error message and publishes an SSE event.
func (s *SessionService) UpdateErrorMessage(ctx context.Context, projectID, sessionID string, errorMsg *string) (*Session, error) {
	if err := s.store.UpdateSessionErrorMessage(ctx, sessionID, errorMsg); err != nil {
		return nil, fmt.Errorf("failed to update session error message: %w", err)
	}

	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if s.eventBroker != nil {
		if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, normalizeSessionStatus(sess.Status), ""); err != nil {
			log.Printf("Failed to publish session update event: %v", err)
		}
	}

	return s.mapSession(sess), nil
}

// UpdateSession updates a session and publishes an SSE event.
func (s *SessionService) UpdateSession(ctx context.Context, sessionID, name string, displayName *string, status string) (*Session, error) {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if name != "" {
		sess.Name = name
	}
	if displayName != nil {
		// Treat empty string as null (clear the displayName)
		if *displayName == "" {
			sess.DisplayName = nil
		} else {
			sess.DisplayName = displayName
		}
	}
	if status != "" {
		sess.Status = normalizeSessionStatus(status)
	}
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return nil, fmt.Errorf("failed to update session: %w", err)
	}

	if s.eventBroker != nil {
		if err := s.eventBroker.PublishSessionUpdated(ctx, sess.ProjectID, sessionID, sess.Status, sess.CommitStatus); err != nil {
			log.Printf("Failed to publish session update event: %v", err)
		}
	}

	return s.mapSession(sess), nil
}

// ClearTerminalCommitState resets terminal commit metadata for a session that is
// moving back into active editing/chat work.
func (s *SessionService) ClearTerminalCommitState(ctx context.Context, projectID, sessionID string) error {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("failed to get session: %w", err)
	}
	if sess.CommitStatus != model.CommitStatusCompleted && sess.CommitStatus != model.CommitStatusFailed {
		return nil
	}

	sess.CommitStatus = model.CommitStatusNone
	sess.CommitOperation = nil
	sess.CommitError = nil
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to clear commit state: %w", err)
	}

	s.publishCommitStatusChanged(ctx, projectID, sessionID, model.CommitStatusNone)
	return nil
}

// DeleteSession initiates async deletion of a session.
// It sets the session status to "removing", emits an SSE event, and enqueues a deletion job.
func (s *SessionService) DeleteSession(ctx context.Context, projectID, sessionID string, jobQueue JobEnqueuer) error {
	// Get session to verify it exists
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	wasCreateFailed := sess.Status == model.SessionStatusCreateFailed
	if sess.Status != model.SessionStatusRemoving {
		// Update status to "removing"
		sess.Status = model.SessionStatusRemoving
		if err := s.store.UpdateSession(ctx, sess); err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}

		// Emit SSE event
		if s.eventBroker != nil {
			if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, model.SessionStatusRemoving, sess.CommitStatus); err != nil {
				log.Printf("Failed to publish session removing event: %v", err)
			}
		}
	}

	// Enqueue deletion job. Re-enqueue even when the session is already in
	// "removing" so a failed or exhausted prior delete job does not leave the
	// session stuck forever.
	if err := jobQueue.Enqueue(ctx, jobs.SessionDeletePayload{ProjectID: projectID, SessionID: sessionID, CreateFailed: wasCreateFailed}); err != nil {
		// If job enqueueing fails, log but don't fail - the session is marked as removing
		// and can be cleaned up later by reconciliation
		log.Printf("Failed to enqueue session delete job for %s: %v", sessionID, err)
	}

	return nil
}

// CommitSession initiates async commit of a session.
func (s *SessionService) CommitSession(ctx context.Context, projectID, sessionID string, jobQueue JobEnqueuer, opts CommitSessionOptions) error {
	// Get session to verify it exists and get workspace ID
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	if err := s.markSessionOperationPending(ctx, projectID, sess, CommitOperationCommit); err != nil {
		return err
	}

	if err = jobQueue.Enqueue(ctx, jobs.SessionCommitPayload{
		ProjectID:           projectID,
		SessionID:           sessionID,
		WorkspaceID:         sess.WorkspaceID,
		RequestedDirectory:  opts.RequestedDirectory,
		RequestedBaseCommit: opts.RequestedBaseCommit,
		RequestedCommitHash: opts.RequestedCommitHash,
		ApprovalThreadID:    opts.ApprovalThreadID,
		ApprovalQuestionID:  opts.ApprovalQuestionID,
	}); err != nil {
		return fmt.Errorf("failed to enqueue commit job: %w", err)
	}

	return nil
}

func (s *SessionService) markSessionOperationPending(ctx context.Context, projectID string, sess *model.Session, operation string) error {
	if sess.CommitStatus == model.CommitStatusPending || sess.CommitStatus == model.CommitStatusCommitting {
		return ErrSessionOperationInProgress
	}

	sess.CommitStatus = model.CommitStatusPending
	sess.CommitOperation = ptrString(operation)
	sess.CommitError = nil
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session commit status: %w", err)
	}
	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusPending)

	return nil
}

// publishCommitStatusChanged publishes an SSE event for commit status changes.
func (s *SessionService) publishCommitStatusChanged(ctx context.Context, projectID, sessionID, commitStatus string) {
	if s.eventBroker != nil {
		// Send empty string for session status since only commit status changed
		if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, "", commitStatus); err != nil {
			log.Printf("Failed to publish session commit status event: %v", err)
		}
	}
}

// ReconcileCommitStates checks sessions stuck in pending/committing commit states
// and re-enqueues commit jobs if needed. This should be called on server startup.
func (s *SessionService) ReconcileCommitStates(ctx context.Context) error {
	// Query sessions with stuck commit states
	statuses := []string{model.CommitStatusPending, model.CommitStatusCommitting}
	sessions, err := s.store.ListSessionsByCommitStatuses(ctx, statuses)
	if err != nil {
		return fmt.Errorf("failed to list sessions with commit states: %w", err)
	}

	if len(sessions) == 0 {
		log.Println("No sessions with stuck commit states found")
		return nil
	}

	log.Printf("Reconciling %d sessions with stuck commit states", len(sessions))

	// For each session, check if an active job for the same serialized resource already exists.
	var enqueuedCount int
	for _, sess := range sessions {
		operation := CommitOperationCommit
		if sess.CommitOperation != nil && *sess.CommitOperation != "" {
			operation = *sess.CommitOperation
		}

		if operation != CommitOperationCommit {
			log.Printf("Clearing stale non-commit operation state for session %s (operation=%q status=%q)", sess.ID, operation, sess.CommitStatus)
			sess.CommitStatus = model.CommitStatusNone
			sess.CommitOperation = nil
			sess.CommitError = nil
			if err := s.store.UpdateSession(ctx, sess); err != nil {
				log.Printf("Failed to clear stale operation state for session %s: %v", sess.ID, err)
				continue
			}
			s.publishCommitStatusChanged(ctx, sess.ProjectID, sess.ID, model.CommitStatusNone)
			continue
		}

		hasJob, err := s.store.HasActiveJobForResource(ctx, jobs.ResourceTypeWorkspace, sess.WorkspaceID)
		if err != nil {
			log.Printf("Failed to check job for session %s: %v", sess.ID, err)
			continue
		}

		if hasJob {
			log.Printf("Session %s (commit_status: %s) already has active job, skipping", sess.ID, sess.CommitStatus)
			continue
		}

		log.Printf("Re-enqueueing commit job for session %s (commit_status: %s)", sess.ID, sess.CommitStatus)
		payload := jobs.SessionCommitPayload{
			ProjectID:   sess.ProjectID,
			SessionID:   sess.ID,
			WorkspaceID: sess.WorkspaceID,
		}

		if s.jobEnqueuer != nil {
			if err := s.jobEnqueuer.Enqueue(ctx, payload); err != nil {
				// Log but continue - this session remains stuck but others proceed
				log.Printf("Failed to enqueue %s job for session %s: %v", operation, sess.ID, err)
				continue
			}
			enqueuedCount++
		} else {
			log.Printf("Job enqueuer not available for session %s, skipping", sess.ID)
		}
	}

	log.Printf("Reconciled commit states: %d jobs re-enqueued", enqueuedCount)
	return nil
}

// PerformDeletion performs the actual session deletion work.
// This is called by the SessionDeleteExecutor job handler.
func (s *SessionService) PerformDeletion(ctx context.Context, projectID, sessionID string) error {
	return s.performDeletion(ctx, projectID, sessionID, false)
}

func (s *SessionService) PerformDeletionFromDeleteJob(ctx context.Context, projectID, sessionID string, createFailed bool) error {
	return s.performDeletion(ctx, projectID, sessionID, createFailed)
}

func (s *SessionService) performDeletion(ctx context.Context, projectID, sessionID string, createFailed bool) error {
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}
	removeSandboxNow := createFailed || sess.Status == model.SessionStatusCreateFailed

	// Step 1: Stop the sandbox immediately so the session is inactive during the
	// recovery window, but keep the sandbox itself for deferred deletion. If
	// sandbox creation failed, remove it immediately instead; there is no useful
	// sandbox to retain, and remote providers may have reserved a VM that needs an
	// explicit provisioner delete to avoid orphaning it.
	if s.sandboxService != nil {
		if removeSandboxNow {
			if err := s.removeSandboxForDeletedSession(ctx, sessionID); err != nil {
				return err
			}
		} else {
			err := s.sandboxService.StopForSession(ctx, sessionID)
			if err != nil {
				if !errors.Is(err, sandbox.ErrNotFound) && !errors.Is(err, sandbox.ErrNotRunning) {
					log.Printf("Failed to stop sandbox for deleted session %s; continuing with database deletion and deferred cleanup: %v", sessionID, err)
				}
			}
		}
	}

	// Step 2: Delete from database (messages, terminal history, session)
	if err := s.store.DeleteSession(ctx, sessionID); err != nil {
		return fmt.Errorf("failed to delete session from database: %w", err)
	}

	// Step 3: Schedule deferred sandbox cleanup unless the sandbox was already
	// removed because creation failed.
	if !removeSandboxNow && s.jobEnqueuer != nil && s.sandboxService != nil {
		if err := s.jobEnqueuer.Enqueue(ctx, jobs.SessionSandboxDeletePayload{
			SessionID: sessionID,
			DeleteAt:  time.Now().Add(s.cleanupDelay),
		}); err != nil {
			log.Printf("Failed to enqueue deferred session sandbox delete job for %s: %v", sessionID, err)
		}
	}

	// Step 4: Emit "removed" event to notify clients
	if s.eventBroker != nil {
		if err := s.eventBroker.PublishSessionUpdated(ctx, projectID, sessionID, model.SessionStatusRemoved, ""); err != nil {
			log.Printf("Failed to publish session removed event: %v", err)
		}
	}

	log.Printf("Session %s deleted successfully", sessionID)
	return nil
}

func (s *SessionService) removeSandboxForDeletedSession(ctx context.Context, sessionID string) error {
	err := s.sandboxService.RemoveForDeletedSession(ctx, sessionID)
	if err == nil {
		return nil
	}
	return fmt.Errorf("failed to remove failed sandbox: %w", err)
}

// PerformDeferredSandboxDeletion removes a retained sandbox after the
// session has been deleted long enough to age out of the recovery window.
func (s *SessionService) PerformDeferredSandboxDeletion(ctx context.Context, sessionID string) error {
	if s.sandboxService == nil {
		return nil
	}

	if _, err := s.store.GetSessionByID(ctx, sessionID); err == nil {
		log.Printf("Skipping deferred sandbox cleanup for session %s because the session exists again", sessionID)
		return nil
	} else if !errors.Is(err, store.ErrNotFound) {
		return fmt.Errorf("failed to check session before deferred sandbox cleanup: %w", err)
	}

	err := s.sandboxService.RemoveForSession(ctx, sessionID)
	if err != nil && !errors.Is(err, sandbox.ErrNotFound) {
		return fmt.Errorf("failed to remove deferred sandbox: %w", err)
	}

	log.Printf("Deferred sandbox cleanup completed for session %s", sessionID)
	return nil
}

func (s *SessionService) mapSession(sess *model.Session) *Session {
	return s.mapSessionWithActivity(sess, sessionThreadStatusFromModel(sess))
}

// mapSession maps a model Session to a service Session
func (s *SessionService) mapSessionWithActivity(sess *model.Session, activity *SessionActivityStatus) *Session {
	description := ""
	if sess.Description != nil {
		description = *sess.Description
	}

	displayName := ""
	if sess.DisplayName != nil {
		displayName = *sess.DisplayName
	}

	errorMessage := ""
	if sess.ErrorMessage != nil {
		errorMessage = *sess.ErrorMessage
	}

	commitError := ""
	if sess.CommitError != nil {
		commitError = *sess.CommitError
	}

	commitOperation := ""
	if sess.CommitOperation != nil {
		commitOperation = *sess.CommitOperation
	}

	targetRef := defaultSessionTargetRef
	if sess.TargetRef != nil && strings.TrimSpace(*sess.TargetRef) != "" {
		targetRef = strings.TrimSpace(*sess.TargetRef)
	}

	appliedCommit := ""
	if sess.AppliedCommit != nil {
		appliedCommit = *sess.AppliedCommit
	}

	workspacePath := ""
	if sess.WorkspacePath != nil {
		workspacePath = *sess.WorkspacePath
	}

	timestamp := sess.UpdatedAt.Format(time.RFC3339)
	if sess.UpdatedAt.IsZero() {
		timestamp = time.Now().Format(time.RFC3339)
	}

	createdAt := sess.CreatedAt.Format(time.RFC3339)
	if sess.CreatedAt.IsZero() {
		createdAt = timestamp
	}

	session := &Session{
		ID:              sess.ID,
		ProjectID:       sess.ProjectID,
		ProviderID:      sess.SandboxProviderID,
		Name:            sess.Name,
		DisplayName:     displayName,
		Description:     description,
		CreatedAt:       createdAt,
		Timestamp:       timestamp,
		Status:          normalizeSessionStatus(sess.Status),
		CommitStatus:    sess.CommitStatus,
		CommitOperation: commitOperation,
		CommitError:     commitError,
		TargetRef:       targetRef,
		AppliedCommit:   appliedCommit,
		ErrorMessage:    errorMessage,
		Files:           []FileNode{},
		WorkspaceID:     sess.WorkspaceID,
		WorkspacePath:   workspacePath,
	}
	session.ThreadStatus = activity
	return session
}

func sessionThreadStatusFromModel(sess *model.Session) *SessionActivityStatus {
	if sess == nil {
		return nil
	}
	switch normalizeSessionStatus(sess.Status) {
	case model.SessionStatusReady:
		return sessionActivityStatusFromStoredStatus(sess.ThreadStatus)
	default:
		return nil
	}
}

func trimStringPtr(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func sessionTargetRef(sess *model.Session) string {
	if sess == nil {
		return defaultSessionTargetRef
	}
	targetRef := trimStringPtr(sess.TargetRef)
	if targetRef == "" {
		return defaultSessionTargetRef
	}
	return targetRef
}

func ensureSessionTargetRef(sess *model.Session) bool {
	if sess == nil {
		return false
	}
	if trimStringPtr(sess.TargetRef) != "" {
		return false
	}
	sess.TargetRef = ptrString(defaultSessionTargetRef)
	return true
}

func resolveWorkspaceTargetCommit(ctx context.Context, gitService *GitService, workspaceID, targetRef string) (string, error) {
	if gitService == nil {
		return "", fmt.Errorf("git service is unavailable")
	}
	targetRef = strings.TrimSpace(targetRef)
	if targetRef == "" {
		targetRef = defaultSessionTargetRef
	}
	if targetRef != defaultSessionTargetRef {
		return "", fmt.Errorf("unsupported target ref %q", targetRef)
	}
	gitStatus, err := gitService.Status(ctx, workspaceID)
	if err != nil {
		return "", fmt.Errorf("resolve target ref %q: %w", targetRef, err)
	}
	if gitStatus == nil || strings.TrimSpace(gitStatus.Commit) == "" {
		return "", fmt.Errorf("resolve target ref %q: workspace commit is unavailable", targetRef)
	}
	return strings.TrimSpace(gitStatus.Commit), nil
}

func resolveSessionTargetCommit(ctx context.Context, gitService *GitService, sess *model.Session) (string, error) {
	if sess == nil {
		return "", fmt.Errorf("session is unavailable")
	}
	return resolveWorkspaceTargetCommit(ctx, gitService, sess.WorkspaceID, sessionTargetRef(sess))
}

// SessionConfig contains all the configuration needed for agent startup.
type SessionConfig struct {
	Session   *Session   `json:"session"`
	Workspace *Workspace `json:"workspace"`
}

// Initialize performs the session initialization work synchronously.
// This is called by the dispatcher when processing a session_init job.
// The session must already exist in the database.
func (s *SessionService) Initialize(
	ctx context.Context,
	sessionID string,
) error {
	// Get session from store (model)
	sessionModel, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Get workspace info
	workspace, err := s.store.GetWorkspaceByID(ctx, sessionModel.WorkspaceID)
	if err != nil {
		s.updateStatusWithEvent(ctx, sessionModel.ProjectID, sessionID, model.SessionStatusError, ptrString("workspace not found: "+err.Error()))
		return fmt.Errorf("workspace not found: %w", err)
	}

	// Convert to service Session for initializeSync
	session := s.mapSession(sessionModel)

	// Run initialization synchronously
	return s.initializeSync(ctx, session.ProjectID, session, workspace)
}

// initializeSync runs the initialization flow synchronously.
// The flow is: ensure workspace -> save workspace info on session -> create sandbox.
func (s *SessionService) initializeSync(
	ctx context.Context,
	projectID string,
	session *Session,
	workspace *model.Workspace,
) error {
	sessionID := session.ID
	if s.sandboxService == nil {
		return fmt.Errorf("sandbox service is not configured")
	}

	// Step 1: Ensure workspace is available (always needed, even on reconcile)
	var workspacePath string
	var currentCommit string

	isGitWorkspace := workspace.SourceType == model.WorkspaceSourceTypeGit || git.IsGitURL(workspace.Path)
	cloneInSandbox := isGitWorkspace && git.IsGitURL(workspace.Path) && s.sandboxService.ClonesGitWorkspaces()

	if cloneInSandbox {
		s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusCloning, nil)
	} else if s.gitService != nil {
		if isGitWorkspace {
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusCloning, nil)
		}

		var err error
		workspacePath, currentCommit, err = s.gitService.EnsureWorkspaceRepo(ctx, workspace.ID)
		if err != nil {
			log.Printf("Git setup failed for session %s: %v", sessionID, err)
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("git setup failed: "+err.Error()))
			return fmt.Errorf("git setup failed: %w", err)
		}
	} else {
		// No git service - use workspace path directly (fallback for testing)
		workspacePath = workspace.Path
	}

	// Step 2: Determine which workspace bootstrap inputs to use for the sandbox.
	// Sessions track the merge target by ref, not by storing a rolling base SHA.
	// When the sandbox is created or recreated, always clone the workspace's
	// current target commit.
	workspaceCommit := currentCommit
	if session.WorkspacePath != "" && !cloneInSandbox {
		// Already initialized - keep the existing workspace path.
		workspacePath = session.WorkspacePath
	}
	if err := s.store.UpdateSessionWorkspace(ctx, sessionID, workspacePath, session.TargetRef); err != nil {
		log.Printf("Failed to update session workspace info for %s: %v", sessionID, err)
		s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("failed to save workspace info: "+err.Error()))
		return fmt.Errorf("failed to save workspace info: %w", err)
	}

	// Step 3: Create or get existing sandbox (idempotent)
	// First check if sandbox already exists (from a previous failed attempt)
	var err error
	existingSandbox, err := s.sandboxService.GetSandbox(ctx, sessionID)
	if err != nil && !errors.Is(err, sandbox.ErrNotFound) {
		log.Printf("Failed to check for existing sandbox for session %s: %v", sessionID, err)
		s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("failed to check sandbox: "+err.Error()))
		return fmt.Errorf("failed to check sandbox: %w", err)
	}

	needsCreation := true
	needsHealthCheck := false
	if existingSandbox != nil {
		log.Printf("Sandbox already exists for session %s (status: %s)", sessionID, existingSandbox.Status)

		switch existingSandbox.Status {
		case sandbox.StatusRunning:
			// Container reports running — verify services are actually responsive.
			// This prevents marking a session "ready" when the container is mid-shutdown
			// (e.g., idle monitor sent SIGTERM but Docker still reports "running").
			if err := s.sandboxService.probeSandboxHealth(ctx, sessionID); err != nil {
				log.Printf("Sandbox for session %s reports running but health check failed: %v, removing", sessionID, err)
				if rmErr := s.sandboxService.RemoveSessionSandbox(ctx, sessionID); rmErr != nil {
					log.Printf("Failed to remove unhealthy sandbox for session %s: %v", sessionID, rmErr)
				}
				needsCreation = true
				break
			}
			log.Printf("Sandbox for session %s is already running (verified healthy)", sessionID)
			needsCreation = false

		case sandbox.StatusCreated, sandbox.StatusStopped:
			if err := s.sandboxService.WaitForSandboxImageOpsReady(ctx); err != nil {
				log.Printf("Warning: failed to wait for sandbox image provider readiness for session %s: %v", sessionID, err)
			}

			expectedImageID, err := s.sandboxService.CurrentSandboxImageID(ctx)
			if err != nil {
				log.Printf("Warning: failed to resolve current sandbox image ID for session %s: %v", sessionID, err)
			}
			expectedImage := s.sandboxService.SandboxImage()

			if !sandboxUsesExpectedImage(existingSandbox, expectedImage, expectedImageID) {
				log.Printf("Sandbox for session %s is inactive but uses outdated image %s (expected %s), recreating instead of restarting",
					sessionID, existingSandbox.Image, expectedImage)
				if err := s.sandboxService.RemoveSessionSandbox(ctx, sessionID); err != nil {
					log.Printf("Failed to remove outdated sandbox for session %s: %v", sessionID, err)
					s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("failed to remove outdated sandbox: "+err.Error()))
					return fmt.Errorf("failed to remove outdated sandbox: %w", err)
				}
				needsCreation = true
				break
			}

			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusCreatingSandbox, nil)
			err = s.sandboxService.StartSessionSandbox(ctx, sessionID)
			if err != nil {
				if !errors.Is(err, sandbox.ErrAlreadyRunning) {
					log.Printf("Sandbox start failed for session %s: %v, will attempt to remove and recreate", sessionID, err)
					// Start failed - try to remove and recreate
					if rmErr := s.sandboxService.RemoveSessionSandbox(ctx, sessionID); rmErr != nil {
						log.Printf("Failed to remove failed sandbox for session %s: %v", sessionID, rmErr)
						s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("sandbox start failed and removal failed: "+rmErr.Error()))
						return fmt.Errorf("sandbox start failed and removal failed: %w", rmErr)
					}
					// Successfully removed, allow recreation
					needsCreation = true
				} else {
					// Already running (race condition), treat as success
					needsCreation = false
					needsHealthCheck = true
				}
			} else {
				// Successfully started
				needsCreation = false
				needsHealthCheck = true
			}

		default:
			// Sandbox is in failed state - remove and recreate (preserve volumes)
			log.Printf("Removing failed sandbox for session %s", sessionID)
			if err := s.sandboxService.RemoveSessionSandbox(ctx, sessionID); err != nil {
				log.Printf("Failed to remove old sandbox for session %s: %v", sessionID, err)
				s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("failed to remove old sandbox: "+err.Error()))
				return fmt.Errorf("failed to remove old sandbox: %w", err)
			}
		}
	}

	if needsCreation {
		// Check if image needs to be pulled and notify if so
		if !s.sandboxService.SandboxImageExists(ctx) {
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusPullingImage, nil)
			log.Printf("Pulling sandbox image %s for session %s", s.sandboxService.SandboxImage(), sessionID)
		} else {
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusCreatingSandbox, nil)
		}

		sandboxSecret := generateSecret(32)
		mcpOAuthRedirectBase := s.sandboxService.cfg.MCPOAuthRedirectBase
		agentServerURL := s.sandboxService.cfg.AgentServerURL
		opts := sandbox.CreateOptions{
			SharedSecret: sandboxSecret,
			Env:          sandboxCreateEnv(sessionID, sandboxSecret, workspacePath, workspace.Path, workspaceCommit, session.TargetRef, projectID, mcpOAuthRedirectBase, agentServerURL),
			Labels: map[string]string{
				"discobot.session.id":   sessionID,
				"discobot.workspace.id": workspace.ID,
				"discobot.project.id":   projectID,
			},
			WorkspacePath:      workspacePath,
			WorkspaceSource:    workspace.Path,
			WorkspaceCommit:    workspaceCommit,
			WorkspaceTargetRef: session.TargetRef,
		}

		if err := s.sandboxService.PrepareSessionSandboxState(ctx, sessionID, opts); err != nil {
			log.Printf("Sandbox state preparation failed for session %s: %v", sessionID, err)
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("sandbox state preparation failed: "+err.Error()))
			return fmt.Errorf("sandbox state preparation failed: %w", err)
		}

		if err := s.sandboxService.CreateSessionSandbox(ctx, sessionID, opts); err != nil {
			log.Printf("Sandbox creation failed for session %s: %v", sessionID, err)
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusCreateFailed, ptrString("sandbox creation failed: "+err.Error()))
			return fmt.Errorf("sandbox creation failed: %w", err)
		}

		// Start the sandbox
		if err := s.sandboxService.StartSessionSandbox(ctx, sessionID); err != nil {
			log.Printf("Sandbox start failed for session %s: %v", sessionID, err)
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("sandbox start failed: "+err.Error()))
			return fmt.Errorf("sandbox start failed: %w", err)
		}
		needsHealthCheck = true
	}

	if needsHealthCheck && s.sandboxService != nil {
		if err := s.sandboxService.waitForSandboxHealth(ctx, sessionID); err != nil {
			log.Printf("Sandbox health check failed for session %s after start: %v", sessionID, err)
			s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusError, ptrString("sandbox health check failed: "+err.Error()))
			return fmt.Errorf("sandbox health check failed: %w", err)
		}
	}

	// Success! Update status to running
	s.updateStatusWithEvent(ctx, projectID, sessionID, model.SessionStatusReady, nil)
	log.Printf("Session %s initialized successfully", sessionID)
	return nil
}

// updateStatusWithEvent updates session status and emits an SSE event.
// Initialization jobs may still be running when deletion starts; never let them
// move a session out of the removing/removed terminal cleanup path.
func (s *SessionService) updateStatusWithEvent(ctx context.Context, projectID, sessionID, status string, errorMsg *string) {
	current, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		log.Printf("Failed to load session %s before status update to %s: %v", sessionID, status, err)
		return
	}
	if (current.Status == model.SessionStatusRemoving || current.Status == model.SessionStatusRemoved) &&
		status != model.SessionStatusRemoving && status != model.SessionStatusRemoved {
		log.Printf("Skipping session %s status update to %s because current status is %s", sessionID, status, current.Status)
		return
	}

	_, err = s.UpdateStatus(ctx, projectID, sessionID, status, errorMsg)
	if err != nil {
		log.Printf("Failed to update session %s status to %s: %v", sessionID, status, err)
	}
}

// generateSecret generates a cryptographically secure random hex string.
func generateSecret(length int) string {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less random but still unique value
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(bytes)
}

// ptrString returns a pointer to a string.
func ptrString(s string) *string {
	return new(s)
}

// PerformCommit performs the session commit work synchronously.
// This is called by the dispatcher when processing a session_commit job.
// Jobs for the same workspace are serialized by the job queue, so no
// precondition checks on commit status are needed.
//
// Flow:
// 1. Set session commit state and ensure the target ref is present
// 2. If patches are already available relative to the current target, apply them
// 3. If pending: send /discobot-commit to agent, transition to committing
// 4. If appliedCommit not set: resolve the current target, fetch patches, and apply them
// 5. Transition to completed
func (s *SessionService) PerformCommit(ctx context.Context, projectID, sessionID string, opts CommitSessionOptions) (retErr error) {
	// Get session
	sess, err := s.store.GetSessionByID(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	// Get workspace
	workspace, err := s.store.GetWorkspaceByID(ctx, sess.WorkspaceID)
	if err != nil {
		return fmt.Errorf("workspace not found: %w", err)
	}

	// If PerformCommit returns an error (e.g. context deadline exceeded),
	// mark the commit as failed so it doesn't get stuck in "pending" or "committing".
	// Use a background context since the original ctx may have been cancelled.
	defer func() {
		if retErr != nil {
			failCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			s.setCommitFailed(failCtx, projectID, workspace, sess, retErr.Error())
			retErr = nil
		}
	}()

	if ensureSessionTargetRef(sess) {
		if err := s.store.UpdateSession(ctx, sess); err != nil {
			return fmt.Errorf("failed to backfill session target ref: %w", err)
		}
	}

	sess.CommitStatus = model.CommitStatusPending
	sess.CommitOperation = ptrString(CommitOperationCommit)
	sess.AppliedCommit = nil
	sess.CommitError = nil
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session for commit: %w", err)
	}
	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusPending)

	// Step 1: Optimistically check if the agent already has patches ready.
	if sess.CommitStatus == model.CommitStatusPending && (sess.AppliedCommit == nil || *sess.AppliedCommit == "") {
		if err := s.tryApplyExistingPatches(ctx, projectID, workspace, sess, opts); err != nil {
			return err
		}
		if sess.CommitStatus == model.CommitStatusFailed {
			return nil
		}
	}

	// Step 2: Send /discobot-commit to agent (if pending)
	if sess.CommitStatus == model.CommitStatusPending && !usesPreparedSandboxCommitPull(opts) {
		if err := s.sendCommitPrompt(ctx, projectID, workspace, sess); err != nil {
			return err
		}
		if sess.CommitStatus == model.CommitStatusFailed {
			return nil
		}
	}

	// Step 3: Fetch and apply patches (if not yet done)
	if sess.AppliedCommit == nil || *sess.AppliedCommit == "" {
		if err := s.fetchAndApplyPatches(ctx, projectID, workspace, sess, opts); err != nil {
			return err
		}
		if sess.CommitStatus == model.CommitStatusFailed {
			return nil
		}
	}

	// Step 4: Complete
	appliedCommit := ""
	if sess.AppliedCommit != nil {
		appliedCommit = *sess.AppliedCommit
	}
	log.Printf("Session %s: commit completed with applied commit %s", sess.ID, appliedCommit)
	if err := s.markCommitCompleted(ctx, projectID, sess); err != nil {
		return err
	}

	log.Printf("Workspace %s committed successfully via session %s", workspace.ID, sess.ID)
	return nil
}

// tryApplyExistingPatches checks if the agent already has patches ready and applies them.
// This is called optimistically before sending /discobot-commit in case commits are already available.
func (s *SessionService) tryApplyExistingPatches(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session, opts CommitSessionOptions) error {
	if s.sandboxService == nil {
		return nil
	}

	targetCommit := ""
	var err error
	if !usesPreparedSandboxCommitPull(opts) {
		targetCommit, err = resolveSessionTargetCommit(ctx, s.gitService, sess)
		if err != nil {
			s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to resolve session target commit: %v", err))
			return nil
		}
		log.Printf("Session %s: checking if agent has existing patches for target %s (%s)", sess.ID, sessionTargetRef(sess), targetCommit)
	} else {
		log.Printf("Session %s: checking if agent has prepared sandbox commits for requested head %s", sess.ID, opts.RequestedCommitHash)
	}

	client, err := s.sandboxService.GetClient(ctx, sess.ID)
	if err != nil {
		if usesPreparedSandboxCommitPull(opts) {
			s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to load prepared sandbox commits: %v", err))
			return nil
		}
		log.Printf("Session %s: no existing patches available (error: %v), continuing with prompt", sess.ID, err)
		return nil
	}

	commitsResp, err := client.GetCommits(ctx, GetCommitsRequest{
		TargetCommit: chooseRequestedBaseCommit(targetCommit, opts),
		HeadCommit:   opts.RequestedCommitHash,
		Directory:    opts.RequestedDirectory,
	})
	if err != nil {
		if usesPreparedSandboxCommitPull(opts) {
			s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to load prepared sandbox commits: %v", err))
			return nil
		}
		log.Printf("Session %s: no existing patches available (error: %v), continuing with prompt", sess.ID, err)
		return nil
	}
	if commitsResp.CommitCount == 0 {
		if usesPreparedSandboxCommitPull(opts) {
			s.setCommitFailed(ctx, projectID, workspace, sess, "Prepared sandbox commits are no longer available to pull")
			return nil
		}
		log.Printf("Session %s: no existing patches available (commit count: 0), continuing with prompt", sess.ID)
		return nil
	}
	if err := validateRequestedCommitHash(opts.RequestedCommitHash, commitsResp.HeadCommit); err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, err.Error())
		return nil
	}

	// Agent already has commits ready - apply the patches directly
	log.Printf("Session %s: agent has %d existing commits, skipping prompt and applying patches (target=%s resolvedTarget=%s dir=%s requestedCommit=%s actualHead=%s)", sess.ID, commitsResp.CommitCount, sessionTargetRef(sess), targetCommit, opts.RequestedDirectory, opts.RequestedCommitHash, commitsResp.HeadCommit)
	return s.applyPatches(ctx, projectID, workspace, sess, targetCommit, commitsResp.Patches, commitsResp.CommitCount, commitsResp.HeadCommit, opts)
}

// waitForPromptTerminalEvent waits for a prompt stream to reach a terminal state.
// Newer agent streams stay open across turns, so commit operations must
// stop on terminal chunk types rather than waiting for the SSE connection to close.
func waitForPromptTerminalEvent(ctx context.Context, streamCh <-chan SSELine) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case line, ok := <-streamCh:
			if !ok {
				if err := ctx.Err(); err != nil {
					return err
				}
				return fmt.Errorf("chat stream ended before completion finished")
			}

			if line.Done {
				return nil
			}
			if line.Data == "" {
				continue
			}

			var payload struct {
				Type      string `json:"type"`
				ErrorText string `json:"errorText,omitempty"`
				Reason    string `json:"reason,omitempty"`
			}
			if err := json.Unmarshal([]byte(line.Data), &payload); err != nil {
				continue
			}

			switch payload.Type {
			case "finish":
				return nil
			case "error":
				if payload.ErrorText == "" {
					payload.ErrorText = "agent completion returned an error"
				}
				return errors.New(payload.ErrorText)
			case "abort":
				if payload.Reason == "" {
					payload.Reason = "agent completion aborted"
				}
				return errors.New(payload.Reason)
			}
		}
	}
}

func isPromptStreamInterruption(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	message := err.Error()
	return strings.Contains(message, "failed to read chat stream:") ||
		strings.Contains(message, "chat stream ended before completion finished")
}

// sendCommitPrompt sends the /discobot-commit command to the agent.
func (s *SessionService) sendCommitPrompt(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session) error {
	if s.sandboxService == nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, "Sandbox service not available")
		return nil
	}

	log.Printf("Session %s: sending /discobot-commit to agent", sess.ID)

	commitMessage := "/discobot-commit"
	messages, err := buildCommitMessage(sess.ID+"-commit", commitMessage)
	if err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to build commit message: %v", err))
		return nil
	}

	client, err := s.sandboxService.GetClient(ctx, sess.ID)
	if err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to get sandbox client: %v", err))
		return nil
	}

	// Dereference model pointer; use empty string if nil (agent will use default)
	modelID := ""
	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	streamCh, err := client.SendMessages(streamCtx, sess.ID, messages, modelID, nil)
	if err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to send commit message to agent: %v", err))
		return nil
	}

	// Transition to committing now that the agent is actively working
	sess.CommitStatus = model.CommitStatusCommitting
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session status: %w", err)
	}
	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusCommitting)

	if err := waitForPromptTerminalEvent(streamCtx, streamCh); err != nil {
		if isPromptStreamInterruption(err) {
			log.Printf("Session %s: commit prompt stream interrupted before terminal chunk (%v), continuing with replay reconciliation", sess.ID, err)
			return nil
		}
		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed while waiting for commit prompt to finish: %v", err))
		return nil
	}

	log.Printf("Session %s: /discobot-commit message completed", sess.ID)
	return nil
}

// fetchAndApplyPatches fetches patches from the agent and applies them to the workspace.
func (s *SessionService) fetchAndApplyPatches(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session, opts CommitSessionOptions) error {
	if s.sandboxService == nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, "Sandbox service not available")
		return nil
	}

	targetCommit := ""
	var err error
	if !usesPreparedSandboxCommitPull(opts) {
		targetCommit, err = resolveSessionTargetCommit(ctx, s.gitService, sess)
		if err != nil {
			s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to resolve session target commit: %v", err))
			return nil
		}
		log.Printf("Session %s: fetching commits from agent-api (target=%s resolvedTarget=%s dir=%s requestedCommit=%s)", sess.ID, sessionTargetRef(sess), targetCommit, opts.RequestedDirectory, opts.RequestedCommitHash)
	} else {
		log.Printf("Session %s: fetching prepared sandbox commits from agent-api (dir=%s requestedCommit=%s)", sess.ID, opts.RequestedDirectory, opts.RequestedCommitHash)
	}

	client, err := s.sandboxService.GetClient(ctx, sess.ID)
	if err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to get sandbox client: %v", err))
		return nil
	}

	commitsResp, err := client.GetCommits(ctx, GetCommitsRequest{
		TargetCommit: chooseRequestedBaseCommit(targetCommit, opts),
		HeadCommit:   opts.RequestedCommitHash,
		Directory:    opts.RequestedDirectory,
	})
	if err != nil {
		var noOp *CommitsNoOpError
		if errors.As(err, &noOp) {
			if usesPreparedSandboxCommitPull(opts) {
				s.setCommitFailed(ctx, projectID, workspace, sess,
					fmt.Sprintf("Prepared sandbox commits are no longer available to pull (head=%s isClean=%v)", noOp.HeadCommit, noOp.IsClean))
				return nil
			}
			if completeErr := s.validateAndCompleteNoOp(ctx, projectID, workspace, sess, noOp, targetCommit, "agent reported no changes relative to the current target"); completeErr != nil {
				return completeErr
			}
			return nil
		}

		s.setCommitFailed(ctx, projectID, workspace, sess, fmt.Sprintf("Failed to get commits from agent: %v", err))
		return nil
	}

	if commitsResp.CommitCount == 0 {
		if usesPreparedSandboxCommitPull(opts) {
			s.setCommitFailed(ctx, projectID, workspace, sess, "Prepared sandbox commits are no longer available to pull")
			return nil
		}
		// Agent returned a success response with zero commits — treat as a dirty no-op and fail.
		s.setCommitFailed(ctx, projectID, workspace, sess, "Agent returned zero commits in patch response without a clean working tree confirmation")
		return nil
	}
	if err := validateRequestedCommitHash(opts.RequestedCommitHash, commitsResp.HeadCommit); err != nil {
		s.setCommitFailed(ctx, projectID, workspace, sess, err.Error())
		return nil
	}

	log.Printf("Session %s: received %d commits from agent, applying patches to workspace (target=%s resolvedTarget=%s dir=%s requestedCommit=%s actualHead=%s)", sess.ID, commitsResp.CommitCount, sessionTargetRef(sess), targetCommit, opts.RequestedDirectory, opts.RequestedCommitHash, commitsResp.HeadCommit)
	return s.applyPatches(ctx, projectID, workspace, sess, targetCommit, commitsResp.Patches, commitsResp.CommitCount, commitsResp.HeadCommit, opts)
}

// applyPatches applies the given patches to the workspace and updates the session.
func (s *SessionService) applyPatches(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session, targetCommit, patches string, commitCount int, sandboxHeadCommit string, opts CommitSessionOptions) error {
	if sess.CommitStatus != model.CommitStatusCommitting {
		sess.CommitStatus = model.CommitStatusCommitting
		if err := s.store.UpdateSession(ctx, sess); err != nil {
			return fmt.Errorf("failed to update session status: %w", err)
		}
		s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusCommitting)
	}

	finalCommit, err := s.gitService.ApplyPatches(ctx, sess.WorkspaceID, []byte(patches))
	if err != nil {
		errorMsg := fmt.Sprintf("Failed to apply patches to workspace: %v", err)
		if usesPreparedSandboxCommitPull(opts) {
			requestedBase := strings.TrimSpace(opts.RequestedBaseCommit)
			currentHead, resolveErr := resolveSessionTargetCommit(ctx, s.gitService, sess)
			if requestedBase != "" && resolveErr == nil && !commitHashesMatch(requestedBase, currentHead) {
				errorMsg = fmt.Sprintf("%s\n\nThe requested patch base %s does not match the current workspace HEAD %s. Rebase the sandbox changes onto the current HEAD, create a new commit, and call RequestCommitPull again instead of creating unrelated replacement commits.", errorMsg, requestedBase, strings.TrimSpace(currentHead))
			}
		}
		s.setCommitFailed(ctx, projectID, workspace, sess, errorMsg)
		return nil
	}

	sess.AppliedCommit = ptrString(finalCommit)
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session applied commit: %w", err)
	}
	if err := s.recordSessionCommitLog(ctx, sess, patches, commitCount, targetCommit, sandboxHeadCommit, finalCommit, opts); err != nil {
		log.Printf("Session %s: failed to record session commit log: %v", sess.ID, err)
	}
	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusCommitting)
	log.Printf("Session %s: %d patches applied, final commit=%s target=%s sandboxHead=%s", sess.ID, commitCount, finalCommit, strings.TrimSpace(targetCommit), strings.TrimSpace(sandboxHeadCommit))
	return nil
}

func (s *SessionService) recordSessionCommitLog(ctx context.Context, sess *model.Session, patches string, commitCount int, targetCommit, sandboxHeadCommit, appliedCommit string, opts CommitSessionOptions) error {
	if s.store == nil || sess == nil {
		return nil
	}

	targetCommit = strings.TrimSpace(targetCommit)
	requestedCommitHash := strings.TrimSpace(opts.RequestedCommitHash)
	requestedDirectory := strings.TrimSpace(opts.RequestedDirectory)
	sandboxHeadCommit = strings.TrimSpace(sandboxHeadCommit)
	appliedCommit = strings.TrimSpace(appliedCommit)

	entry := &model.SessionCommitLog{
		SessionID:   sess.ID,
		Operation:   CommitOperationCommit,
		CommitCount: commitCount,
		Patches:     patches,
	}
	if commitOperation := trimStringPtr(sess.CommitOperation); commitOperation != "" {
		entry.Operation = commitOperation
	}
	if targetRef := sessionTargetRef(sess); targetRef != "" {
		entry.TargetRef = new(targetRef)
	}
	if targetCommit != "" {
		entry.TargetCommit = new(targetCommit)
	}
	if sandboxHeadCommit != "" {
		entry.SandboxHeadCommit = new(sandboxHeadCommit)
	}
	if requestedCommitHash != "" {
		entry.RequestedCommitHash = new(requestedCommitHash)
	}
	if requestedDirectory != "" {
		entry.RequestedDirectory = new(requestedDirectory)
	}
	if appliedCommit != "" {
		entry.AppliedCommit = new(appliedCommit)
	}
	return s.store.CreateSessionCommitLog(ctx, entry)
}

// validateAndCompleteNoOp checks whether a no_commits response from the sandbox is safe
// to treat as a completed no-op. It requires the sandbox working tree to be
// clean so there are no uncommitted changes hidden behind an empty target diff.
func (s *SessionService) validateAndCompleteNoOp(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session, noOp *CommitsNoOpError, targetCommit, logMsg string) error {
	if !noOp.IsClean {
		s.setCommitFailed(ctx, projectID, workspace, sess,
			fmt.Sprintf("Sandbox reports no changes relative to target %s but working tree is dirty (uncommitted changes present); head=%s", strings.TrimSpace(targetCommit), noOp.HeadCommit))
		return nil
	}
	log.Printf("Session %s: %s target=%s (head=%s isClean=true)", sess.ID, logMsg, strings.TrimSpace(targetCommit), noOp.HeadCommit)
	return s.markCommitCompleted(ctx, projectID, sess)
}

func validateRequestedCommitHash(requestedShortHash, actualHead string) error {
	requestedShortHash = strings.TrimSpace(requestedShortHash)
	if requestedShortHash == "" {
		return nil
	}
	actualHead = strings.TrimSpace(actualHead)
	if actualHead == "" {
		return fmt.Errorf("sandbox did not report a head commit while resolving requested commit %s", requestedShortHash)
	}
	if !strings.HasPrefix(actualHead, requestedShortHash) {
		return fmt.Errorf("requested sandbox commit %s does not match sandbox head %s", requestedShortHash, actualHead)
	}
	return nil
}

func chooseRequestedBaseCommit(resolvedTargetCommit string, opts CommitSessionOptions) string {
	if strings.TrimSpace(opts.RequestedBaseCommit) != "" {
		return strings.TrimSpace(opts.RequestedBaseCommit)
	}
	return strings.TrimSpace(resolvedTargetCommit)
}

func usesPreparedSandboxCommitPull(opts CommitSessionOptions) bool {
	return strings.TrimSpace(opts.RequestedCommitHash) != ""
}

func commitHashesMatch(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	return a != "" && b != "" && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
}

func (s *SessionService) FinalizeRequestCommitPullApproval(ctx context.Context, sessionID, threadID, questionID string, commitErr error) error {
	if strings.TrimSpace(threadID) == "" || strings.TrimSpace(questionID) == "" {
		return nil
	}
	if s.sandboxService == nil {
		return fmt.Errorf("sandbox service not available")
	}

	answers := map[string]string{}
	if commitErr != nil {
		answers[requestCommitPullFailedKey] = "true"
		answers[requestCommitPullResultKey] = fmt.Sprintf("Discobot failed to pull the prepared sandbox commit into the host workspace: %v", commitErr)
	} else {
		sess, err := s.store.GetSessionByID(ctx, sessionID)
		if err != nil {
			return fmt.Errorf("load session after commit: %w", err)
		}
		switch sess.CommitStatus {
		case model.CommitStatusCompleted:
			answers[requestCommitPullSucceededKey] = "true"
			if sess.AppliedCommit != nil && strings.TrimSpace(*sess.AppliedCommit) != "" {
				answers[requestCommitPullResultKey] = fmt.Sprintf("Discobot successfully pulled the prepared sandbox commit into the host workspace. Applied workspace commit: %s", strings.TrimSpace(*sess.AppliedCommit))
			} else {
				answers[requestCommitPullResultKey] = "Discobot completed the pull request, but there was no new workspace commit to apply."
			}
		case model.CommitStatusFailed:
			answers[requestCommitPullFailedKey] = "true"
			if sess.CommitError != nil && strings.TrimSpace(*sess.CommitError) != "" {
				answers[requestCommitPullResultKey] = fmt.Sprintf("Discobot failed to pull the prepared sandbox commit into the host workspace: %s", strings.TrimSpace(*sess.CommitError))
			} else {
				answers[requestCommitPullResultKey] = "Discobot failed to pull the prepared sandbox commit into the host workspace."
			}
		default:
			answers[requestCommitPullFailedKey] = "true"
			answers[requestCommitPullResultKey] = fmt.Sprintf("Discobot finished the pull job in unexpected state %q.", sess.CommitStatus)
		}
	}

	client, err := s.sandboxService.GetClient(ctx, sessionID)
	if err != nil {
		return fmt.Errorf("get sandbox client for approval answer: %w", err)
	}
	_, err = client.AnswerQuestion(ctx, threadID, &sandboxapi.AnswerQuestionRequest{
		ToolUseID: questionID,
		Answers:   answers,
	})
	if err != nil {
		return fmt.Errorf("submit request commit pull approval answer: %w", err)
	}
	return nil
}

func (s *SessionService) markCommitCompleted(ctx context.Context, projectID string, sess *model.Session) error {
	ensureSessionTargetRef(sess)

	sess.CommitStatus = model.CommitStatusCompleted
	sess.CommitError = nil
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		return fmt.Errorf("failed to update session commit status: %w", err)
	}
	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusCompleted)
	return nil
}

// setCommitFailed sets the commit status to failed with an error message.
func (s *SessionService) setCommitFailed(ctx context.Context, projectID string, workspace *model.Workspace, sess *model.Session, errorMsg string) {
	log.Printf("Workspace %s commit failed (via session %s): %s", workspace.ID, sess.ID, errorMsg)

	sess.CommitStatus = model.CommitStatusFailed
	sess.CommitError = ptrString(errorMsg)
	if err := s.store.UpdateSession(ctx, sess); err != nil {
		log.Printf("Failed to update session %s commit status to failed: %v", sess.ID, err)
		return
	}

	s.publishCommitStatusChanged(ctx, projectID, sess.ID, model.CommitStatusFailed)
}

// buildCommitMessage creates a UIMessage array for operation commands.
// Returns json.RawMessage that can be passed to SendMessages.
func buildCommitMessage(msgID, text string) (json.RawMessage, error) {
	// Build the text part
	part := map[string]any{
		"type": "text",
		"text": text,
	}
	parts, err := json.Marshal([]any{part})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal parts: %w", err)
	}

	// Build the message
	message := map[string]any{
		"id":    msgID,
		"role":  "user",
		"parts": json.RawMessage(parts),
	}
	messages, err := json.Marshal([]any{message})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal messages: %w", err)
	}

	return messages, nil
}
