package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"regexp"
	"strings"
	"time"

	"github.com/obot-platform/discobot/server/internal/model"
	"github.com/obot-platform/discobot/server/internal/sandbox"
	"github.com/obot-platform/discobot/server/internal/store"
)

// ProjectService handles project operations
type ProjectService struct {
	store    *store.Store
	provider sandbox.Provider
}

// Project represents a project (for API responses)
type Project struct {
	ID                       string    `json:"id"`
	Name                     string    `json:"name"`
	Slug                     string    `json:"slug"`
	DefaultSandboxProviderID string    `json:"defaultSandboxProviderId,omitempty"`
	CreatedAt                time.Time `json:"createdAt"`
	UpdatedAt                time.Time `json:"updatedAt"`
}

// ProjectMember represents a project member (for API responses)
type ProjectMember struct {
	ID         string     `json:"id"`
	ProjectID  string     `json:"projectId"`
	UserID     string     `json:"userId"`
	Role       string     `json:"role"`
	Email      string     `json:"email"`
	Name       string     `json:"name"`
	AvatarURL  string     `json:"avatarUrl,omitempty"`
	InvitedAt  *time.Time `json:"invitedAt,omitempty"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
}

// ProjectInvitation represents a project invitation (for API responses)
type ProjectInvitation struct {
	ID        string    `json:"id"`
	ProjectID string    `json:"projectId"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Token     string    `json:"token,omitempty"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

// ProjectResources describes project-scoped VM resource settings.
type ProjectResources struct {
	Provider string             `json:"provider"`
	VM       ProjectVMResources `json:"vm"`
}

// ProjectVMResources contains the effective VM resources and supported changes.
type ProjectVMResources struct {
	CPUCount                 int  `json:"cpuCount"`
	MemoryMB                 int  `json:"memoryMB"`
	DataDiskGB               int  `json:"dataDiskGB"`
	CanIncreaseDisk          bool `json:"canIncreaseDisk"`
	CanDecreaseDisk          bool `json:"canDecreaseDisk"`
	CanChangeMemory          bool `json:"canChangeMemory"`
	RestartRequiredForDisk   bool `json:"restartRequiredForDisk"`
	RestartRequiredForMemory bool `json:"restartRequiredForMemory"`
}

// UpdateProjectResourcesRequest contains project VM resource changes.
type UpdateProjectResourcesRequest struct {
	MemoryMB   *int `json:"memoryMB,omitempty"`
	DataDiskGB *int `json:"dataDiskGB,omitempty"`
}

// ProjectResourcesUpdateResult describes a project resource update.
type ProjectResourcesUpdateResult struct {
	Provider        string             `json:"provider"`
	Previous        ProjectVMResources `json:"previous"`
	Current         ProjectVMResources `json:"current"`
	RestartRequired bool               `json:"restartRequired"`
}

// ProjectInspection describes inspection-container access for a project.
type ProjectInspection struct {
	Provider      string `json:"provider"`
	Available     bool   `json:"available"`
	ContainerName string `json:"containerName"`
	Scope         string `json:"scope"`
}

// RequestValidationError indicates a request validation failure that should be
// returned to the client as a bad request.
type RequestValidationError struct {
	message string
}

func (e *RequestValidationError) Error() string {
	return e.message
}

func newValidationError(message string) error {
	return &RequestValidationError{message: message}
}

// NewProjectService creates a new project service
func NewProjectService(s *store.Store, p sandbox.Provider) *ProjectService {
	return &ProjectService{
		store:    s,
		provider: p,
	}
}

// ListProjects returns all projects for a user
func (s *ProjectService) ListProjects(ctx context.Context, userID string) ([]Project, error) {
	rows, err := s.store.ListProjectsByUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	projects := make([]Project, len(rows))
	for i, row := range rows {
		projects[i] = Project{
			ID:                       row.ID,
			Name:                     row.Name,
			Slug:                     row.Slug,
			DefaultSandboxProviderID: row.DefaultSandboxProviderID,
			CreatedAt:                row.CreatedAt,
			UpdatedAt:                row.UpdatedAt,
		}
	}
	return projects, nil
}

// CreateProject creates a new project and adds the creator as owner
func (s *ProjectService) CreateProject(ctx context.Context, userID, name string) (*Project, error) {
	slug := generateSlug(name)

	// Create project
	project := &model.Project{
		Name: name,
		Slug: slug,
	}
	if err := s.store.CreateProject(ctx, project); err != nil {
		return nil, fmt.Errorf("failed to create project: %w", err)
	}

	// Add creator as owner
	now := time.Now()
	member := &model.ProjectMember{
		ProjectID:  project.ID,
		UserID:     userID,
		Role:       "owner",
		InvitedBy:  &userID,
		InvitedAt:  &now,
		AcceptedAt: &now,
	}
	if err := s.store.CreateProjectMember(ctx, member); err != nil {
		return nil, fmt.Errorf("failed to add owner: %w", err)
	}

	return &Project{
		ID:                       project.ID,
		Name:                     project.Name,
		Slug:                     project.Slug,
		DefaultSandboxProviderID: project.DefaultSandboxProviderID,
		CreatedAt:                project.CreatedAt,
		UpdatedAt:                project.UpdatedAt,
	}, nil
}

// GetProject returns a project by ID
func (s *ProjectService) GetProject(ctx context.Context, projectID string) (*Project, error) {
	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &Project{
		ID:                       project.ID,
		Name:                     project.Name,
		Slug:                     project.Slug,
		DefaultSandboxProviderID: project.DefaultSandboxProviderID,
		CreatedAt:                project.CreatedAt,
		UpdatedAt:                project.UpdatedAt,
	}, nil
}

// UpdateProject updates a project
func (s *ProjectService) UpdateProject(ctx context.Context, projectID, name string) (*Project, error) {
	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}
	project.Name = name
	if err := s.store.UpdateProject(ctx, project); err != nil {
		return nil, err
	}
	return &Project{
		ID:                       project.ID,
		Name:                     project.Name,
		Slug:                     project.Slug,
		DefaultSandboxProviderID: project.DefaultSandboxProviderID,
		CreatedAt:                project.CreatedAt,
		UpdatedAt:                project.UpdatedAt,
	}, nil
}

// GetProjectResources returns project-scoped VM resources when supported.
func (s *ProjectService) GetProjectResources(ctx context.Context, projectID string) (*ProjectResources, error) {
	return s.GetProjectResourcesForProvider(ctx, projectID, s.provider)
}

// GetProjectResourcesForProvider returns project-scoped VM resources for a specific provider.
func (s *ProjectService) GetProjectResourcesForProvider(ctx context.Context, projectID string, provider sandbox.Provider) (*ProjectResources, error) {
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}

	resourceManager, ok := provider.(sandbox.ProjectResourceManager)
	if !ok {
		return nil, sandbox.ErrProjectResourcesUnsupported
	}

	info, err := resourceManager.GetProjectResourceInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &ProjectResources{
		Provider: info.Provider,
		VM:       projectVMResourcesFromInfo(info),
	}, nil
}

// UpdateProjectResources updates project-scoped VM resources when supported.
func (s *ProjectService) UpdateProjectResources(ctx context.Context, projectID string, req UpdateProjectResourcesRequest) (*ProjectResourcesUpdateResult, error) {
	return s.UpdateProjectResourcesForProvider(ctx, projectID, s.provider, req)
}

// UpdateProjectResourcesForProvider updates project-scoped VM resources for a specific provider.
func (s *ProjectService) UpdateProjectResourcesForProvider(ctx context.Context, projectID string, provider sandbox.Provider, req UpdateProjectResourcesRequest) (*ProjectResourcesUpdateResult, error) {
	if req.MemoryMB == nil && req.DataDiskGB == nil {
		return nil, newValidationError("at least one resource must be provided")
	}

	project, err := s.store.GetProjectByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	resourceManager, ok := provider.(sandbox.ProjectResourceManager)
	if !ok {
		return nil, sandbox.ErrProjectResourcesUnsupported
	}

	currentInfo, err := resourceManager.GetProjectResourceInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}

	if req.MemoryMB != nil {
		if *req.MemoryMB <= 0 {
			return nil, newValidationError("memoryMB must be greater than 0")
		}
		if *req.MemoryMB%1024 != 0 {
			return nil, newValidationError("memoryMB must be a whole GiB multiple")
		}
	}
	if req.DataDiskGB != nil && *req.DataDiskGB <= 0 {
		return nil, newValidationError("dataDiskGB must be greater than 0")
	}
	if req.DataDiskGB != nil && *req.DataDiskGB < currentInfo.DataDiskGB {
		return nil, newValidationError("data disk size can only increase")
	}

	previousMemory := project.VZMemoryMB
	previousDisk := project.VZDataDiskGB
	if req.MemoryMB != nil {
		memoryMB := *req.MemoryMB
		project.VZMemoryMB = &memoryMB
	}
	if req.DataDiskGB != nil {
		dataDiskGB := *req.DataDiskGB
		project.VZDataDiskGB = &dataDiskGB
	}

	if err := s.store.UpdateProject(ctx, project); err != nil {
		return nil, err
	}

	rollback := func() {
		project.VZMemoryMB = previousMemory
		project.VZDataDiskGB = previousDisk
		if updateErr := s.store.UpdateProject(context.Background(), project); updateErr != nil {
			log.Printf("Warning: failed to roll back project resources for %s: %v", projectID, updateErr)
		}
	}

	sandboxReq := sandbox.UpdateProjectResourcesRequest{
		MemoryMB:   req.MemoryMB,
		DataDiskGB: req.DataDiskGB,
	}
	if err := resourceManager.ApplyProjectResourceUpdate(ctx, projectID, sandboxReq); err != nil {
		rollback()
		return nil, err
	}

	updatedInfo, err := resourceManager.GetProjectResourceInfo(ctx, projectID)
	if err != nil {
		rollback()
		return nil, err
	}

	return &ProjectResourcesUpdateResult{
		Provider:        updatedInfo.Provider,
		Previous:        projectVMResourcesFromInfo(currentInfo),
		Current:         projectVMResourcesFromInfo(updatedInfo),
		RestartRequired: true,
	}, nil
}

// GetProjectInspection returns inspection-container access details when supported.
func (s *ProjectService) GetProjectInspection(ctx context.Context, projectID string) (*ProjectInspection, error) {
	return s.GetProjectInspectionForProvider(ctx, projectID, s.provider)
}

// GetProjectInspectionForProvider returns inspection access details for a specific provider.
func (s *ProjectService) GetProjectInspectionForProvider(ctx context.Context, projectID string, provider sandbox.Provider) (*ProjectInspection, error) {
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}

	inspectionManager, ok := provider.(sandbox.ProjectInspectionManager)
	if !ok {
		return nil, sandbox.ErrProjectInspectionUnsupported
	}

	info, err := inspectionManager.GetProjectInspectionInfo(ctx, projectID)
	if err != nil {
		return nil, err
	}

	return &ProjectInspection{
		Provider:      info.Provider,
		Available:     info.Available,
		ContainerName: info.ContainerName,
		Scope:         info.Scope,
	}, nil
}

// AttachProjectInspection attaches to the project's inspection container shell.
func (s *ProjectService) AttachProjectInspection(ctx context.Context, projectID string, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	return s.AttachProjectInspectionForProvider(ctx, projectID, s.provider, opts)
}

// AttachProjectInspectionForProvider attaches to a specific provider's inspection shell.
func (s *ProjectService) AttachProjectInspectionForProvider(ctx context.Context, projectID string, provider sandbox.Provider, opts sandbox.AttachOptions) (sandbox.PTY, error) {
	if _, err := s.store.GetProjectByID(ctx, projectID); err != nil {
		return nil, err
	}

	inspectionManager, ok := provider.(sandbox.ProjectInspectionManager)
	if !ok {
		return nil, sandbox.ErrProjectInspectionUnsupported
	}

	return inspectionManager.AttachProjectInspection(ctx, projectID, opts)
}

// DeleteProject deletes a project and cleans up associated resources
func (s *ProjectService) DeleteProject(ctx context.Context, projectID string) error {
	// Delete from database first
	if err := s.store.DeleteProject(ctx, projectID); err != nil {
		return err
	}

	// Clean up provider-managed resources (cache volumes, BuildKit containers, networks, etc.)
	if s.provider != nil {
		if err := s.provider.RemoveProject(ctx, projectID); err != nil {
			log.Printf("Warning: failed to remove provider resources for project %s: %v", projectID, err)
		}
	}

	return nil
}

// GetMemberRole returns the role of a user in a project
func (s *ProjectService) GetMemberRole(ctx context.Context, projectID, userID string) (string, error) {
	member, err := s.store.GetProjectMember(ctx, projectID, userID)
	if err != nil {
		return "", err
	}
	return member.Role, nil
}

// ListMembers returns all members of a project
func (s *ProjectService) ListMembers(ctx context.Context, projectID string) ([]ProjectMember, error) {
	rows, err := s.store.ListProjectMembers(ctx, projectID)
	if err != nil {
		return nil, err
	}
	members := make([]ProjectMember, len(rows))
	for i, row := range rows {
		member := ProjectMember{
			ID:         row.ID,
			ProjectID:  row.ProjectID,
			UserID:     row.UserID,
			Role:       row.Role,
			InvitedAt:  row.InvitedAt,
			AcceptedAt: row.AcceptedAt,
		}
		// If user is preloaded, add their info
		if row.User != nil {
			member.Email = row.User.Email
			member.Name = ptrToString(row.User.Name)
			member.AvatarURL = ptrToString(row.User.AvatarURL)
		}
		members[i] = member
	}
	return members, nil
}

// CreateInvitation creates a project invitation
func (s *ProjectService) CreateInvitation(ctx context.Context, projectID, inviterID, email, role string) (*ProjectInvitation, error) {
	// Generate token
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	expiresAt := time.Now().Add(7 * 24 * time.Hour) // 7 days

	inv := &model.ProjectInvitation{
		ProjectID: projectID,
		Email:     email,
		Role:      role,
		InvitedBy: &inviterID,
		Token:     token,
		ExpiresAt: expiresAt,
	}
	if err := s.store.CreateInvitation(ctx, inv); err != nil {
		return nil, err
	}
	return &ProjectInvitation{
		ID:        inv.ID,
		ProjectID: inv.ProjectID,
		Email:     inv.Email,
		Role:      inv.Role,
		Token:     inv.Token,
		ExpiresAt: inv.ExpiresAt,
		CreatedAt: inv.CreatedAt,
	}, nil
}

// AcceptInvitation accepts a project invitation
func (s *ProjectService) AcceptInvitation(ctx context.Context, token, userID string) error {
	inv, err := s.store.GetInvitationByToken(ctx, token)
	if err != nil {
		return fmt.Errorf("invitation not found: %w", err)
	}

	if time.Now().After(inv.ExpiresAt) {
		return fmt.Errorf("invitation expired")
	}

	// Add user as member
	now := time.Now()
	member := &model.ProjectMember{
		ProjectID:  inv.ProjectID,
		UserID:     userID,
		Role:       inv.Role,
		InvitedBy:  inv.InvitedBy,
		InvitedAt:  &inv.CreatedAt,
		AcceptedAt: &now,
	}
	if err := s.store.CreateProjectMember(ctx, member); err != nil {
		return fmt.Errorf("failed to add member: %w", err)
	}

	// Delete invitation
	return s.store.DeleteInvitation(ctx, inv.ID)
}

// RemoveMember removes a member from a project
func (s *ProjectService) RemoveMember(ctx context.Context, projectID, userID string) error {
	return s.store.DeleteProjectMember(ctx, projectID, userID)
}

// Helper functions

func generateSlug(name string) string {
	// Convert to lowercase
	slug := strings.ToLower(name)
	// Replace spaces and special chars with hyphens
	reg := regexp.MustCompile(`[^a-z0-9]+`)
	slug = reg.ReplaceAllString(slug, "-")
	// Remove leading/trailing hyphens
	slug = strings.Trim(slug, "-")
	// Add random suffix for uniqueness
	suffix := make([]byte, 4)
	_, _ = rand.Read(suffix)
	return fmt.Sprintf("%s-%s", slug, hex.EncodeToString(suffix))
}

func projectVMResourcesFromInfo(info *sandbox.ProjectResourceInfo) ProjectVMResources {
	if info == nil {
		return ProjectVMResources{}
	}

	return ProjectVMResources{
		CPUCount:                 info.CPUCount,
		MemoryMB:                 info.MemoryMB,
		DataDiskGB:               info.DataDiskGB,
		CanIncreaseDisk:          true,
		CanDecreaseDisk:          false,
		CanChangeMemory:          true,
		RestartRequiredForDisk:   true,
		RestartRequiredForMemory: true,
	}
}
