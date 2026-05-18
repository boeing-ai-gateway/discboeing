package viewmodel

// ShellSnapshot is the read model for the initial full-page render.
type ShellSnapshot struct {
	Header             HeaderSnapshot
	Startup            StartupSnapshot
	Sidebar            AppSidebarSnapshot
	Workspace          SessionWorkspaceSnapshot
	DevErrors          DevErrorOverlaySnapshot
	InspectionTerminal ProjectInspectionTerminalSnapshot
}

// DevErrorOverlaySnapshot describes development-time browser/runtime errors.
type DevErrorOverlaySnapshot struct {
	Enabled  bool
	CopiedID int
	Errors   []DevError
}

// DevError is one captured development error.
type DevError struct {
	ID      int
	Title   string
	Message string
	Stack   string
}

// ProjectInspectionTerminalSnapshot describes the inspection shell dialog.
type ProjectInspectionTerminalSnapshot struct {
	Open             bool
	ProjectID        string
	ProviderID       string
	Title            string
	Description      string
	ConnectionStatus string
	TerminalReady    bool
}

// ProjectSettingsTabSnapshot describes project/provider runtime settings.
type ProjectSettingsTabSnapshot struct {
	Active         bool
	ProviderID     string
	ProviderName   string
	ShowResources  bool
	ShowInspection bool
	Resources      ProjectResourcesSnapshot
	Inspection     ProjectInspectionSnapshot
	Terminal       ProjectInspectionTerminalSnapshot
}

// ProjectResourcesSnapshot describes VM resources controls.
type ProjectResourcesSnapshot struct {
	Status       string
	Error        string
	Provider     string
	CPUCount     int
	MemoryGB     int
	DataDiskGB   int
	MemoryDraft  string
	DiskDraft    string
	SavePending  bool
	SaveError    string
	SaveSuccess  string
	Dirty        bool
	DiskDecrease bool
	ValidDrafts  bool
}

// ProjectInspectionSnapshot describes inspection shell availability.
type ProjectInspectionSnapshot struct {
	Status    string
	Error     string
	Available bool
}

// HeaderSnapshot is the read model for the top application chrome.
type HeaderSnapshot struct {
	ShowSessionToolbar      bool
	SessionTitle            string
	ShowMacWindowSpacer     bool
	ShowRefreshButton       bool
	ShowUpdateBadge         bool
	ShowRightWindowControls bool
	ThreadSwitcher          RecentThreadSwitcherSnapshot
	ShortcutHelp            KeyboardShortcutHelpSnapshot
	SessionToolbar          SessionToolbarSnapshot
	Settings                SettingsDialogSnapshot
}

// SettingsDialogSnapshot describes the global settings dialog.
type SettingsDialogSnapshot struct {
	Open                     bool
	ActiveTab                string
	ShowUpdateTab            bool
	ShowUpdateBadge          bool
	Theme                    string
	ResolvedTheme            string
	ColorScheme              string
	ActiveThemeName          string
	AvailableThemes          []ThemeOption
	RecentThreadsLimit       int
	ShowRefreshButton        bool
	ShowEditorButton         bool
	DefaultModel             string
	Models                   []ModelOption
	ChatWidthMode            string
	AutoScrollOnStream       bool
	Update                   UpdateSnapshot
	CanClearProjectCache     bool
	ClearProjectCacheError   string
	ClearProjectCacheSuccess bool
	Credentials              CredentialsManagerSnapshot
	SandboxProviders         SandboxProvidersManagerSnapshot
	SupportInfo              SupportInfoSnapshot
}

// SupportInfoSnapshot describes the diagnostic support-info dialog.
type SupportInfoSnapshot struct {
	Open   bool
	Status string
	Error  string
	JSON   string
}

// ThemeOption is an available color theme option.
type ThemeOption struct {
	ID   string
	Name string
	Mode string
}

// ModelOption is a model option in settings.
type ModelOption struct {
	ID               string
	Name             string
	Provider         string
	Description      string
	Reasoning        bool
	ReasoningLevels  []string
	DefaultReasoning string
	ServiceTiers     []string
}

// UpdateSnapshot describes app update status in settings.
type UpdateSnapshot struct {
	Status              string
	AvailableVersion    string
	Error               string
	IsIgnored           bool
	TrackPrereleases    bool
	CanTrackPrereleases bool
	DownloadedBytes     int64
	TotalBytes          int64
}

// SessionToolbarSnapshot describes the selected session toolbar in the app header.
type SessionToolbarSnapshot struct {
	SessionID           string
	ActiveView          string
	ShowEditor          bool
	VSCodeAvailable     bool
	DesktopAvailable    bool
	DiffStats           DiffStatsSnapshot
	ServicesCount       int
	PrimaryCommand      ToolbarCommand
	SecondaryCommands   []ToolbarCommand
	Busy                bool
	Pending             bool
	ActiveCommand       string
	PreferredIDE        string
	IDEOptions          []IDEOption
	ShowLearnMoreDialog bool
	CommandCredentials  SessionCommandCredentialsDialogSnapshot
}

// DiffStatsSnapshot is a compact diff summary for toolbar badges.
type DiffStatsSnapshot struct {
	Additions    int
	Deletions    int
	FilesChanged int
}

// ToolbarCommand is a command shown in the session toolbar.
type ToolbarCommand struct {
	Name        string
	Label       string
	ActiveLabel string
	Icon        string
	Group       string
}

// IDEOption is a preferred IDE option in the toolbar menu.
type IDEOption struct {
	ID     string
	Label  string
	Family string
}

// SessionCommandCredentialsDialogSnapshot describes the command credential approval dialog.
type SessionCommandCredentialsDialogSnapshot struct {
	Open         bool
	CommandLabel string
	Error        string
	Requests     []SessionCommandCredentialRequest
}

// SessionCommandCredentialRequest is one requested credential binding.
type SessionCommandCredentialRequest struct {
	EnvVar                string
	Name                  string
	Justification         string
	ApprovedUses          []string
	Options               []SessionCommandCredentialOption
	SelectedOption        string
	SelectedLabel         string
	SelectedDescription   string
	OAuthProviderName     string
	ValidityPreset        string
	ValidityValue         string
	ValidityUnit          string
	CreateCredentialName  string
	CreateCredentialValue string
}

// SessionCommandCredentialOption is a selectable credential binding option.
type SessionCommandCredentialOption struct {
	Value       string
	Label       string
	Description string
	Kind        string
}

// RecentThreadSwitcherSnapshot describes the keyboard thread switcher overlay.
type RecentThreadSwitcherSnapshot struct {
	Open        bool
	Threads     []SidebarThreadItem
	SelectedKey string
	HelpText    string
}

// KeyboardShortcutHelpSnapshot describes the global shortcut help overlay.
type KeyboardShortcutHelpSnapshot struct {
	Open      bool
	Shortcuts []GlobalShortcut
}

// GlobalShortcut is one keyboard shortcut row.
type GlobalShortcut struct {
	ID        string
	Label     string
	KeyGroups [][]string
}

// StartupSnapshot is the read model for startup task banner state.
type StartupSnapshot struct {
	Phase              string
	Ready              bool
	ErrorMessage       string
	RetryCount         int
	LoginHref          string
	OverlayDismissible bool
	VisibleTasks       []StartupTask
	HasActiveTasks     bool
}

// StartupTask mirrors a startup task row shown while app bootstrap work runs.
type StartupTask struct {
	ID               string
	Name             string
	State            string
	Error            string
	CurrentOperation string
	Progress         *int
	BytesDownloaded  *int64
	TotalBytes       *int64
}

// StartupScreenSnapshot describes the standalone startup screen component.
type StartupScreenSnapshot struct {
	ClassName        string
	ModeLabel        string
	MetaLabel        string
	Headline         string
	Detail           string
	StatusLabel      string
	Progress         int
	APIState         string
	RetryCount       int
	ErrorMessage     string
	Steps            []StartupScreenStep
	DetailsOpen      bool
	DismissLabel     string
	Dismissible      bool
	Ready            bool
	ShowShellPreview bool
}

// StartupScreenStep is one startup screen detail row.
type StartupScreenStep struct {
	Label  string
	Detail string
	State  string
}

// AppSidebarSnapshot is the read model for the sessions sidebar.
type AppSidebarSnapshot struct {
	RecentThreads      []SidebarThreadItem
	SessionGroups      []SidebarSessionGroup
	Collapsed          bool
	FloatingOpen       bool
	OpenMenu           SidebarMenuSnapshot
	RenameDialog       SidebarRenameDialogSnapshot
	DeleteDialog       SidebarDeleteDialogSnapshot
	ShowRecentThreads  bool
	RecentOpen         bool
	ShowAllHeader      bool
	AllOpen            bool
	GroupedByWorkspace bool
	StreamEvents       string
	Commands           string
}

// SidebarMenuSnapshot describes the currently open session/thread action menu.
type SidebarMenuSnapshot struct {
	Kind        string
	SessionID   string
	ThreadID    string
	WorkspaceID string
	Title       string
	CanStop     bool
	CanDelete   bool
}

// SidebarRenameDialogSnapshot describes the sidebar rename dialog.
type SidebarRenameDialogSnapshot struct {
	Open        bool
	Kind        string
	SessionID   string
	ThreadID    string
	WorkspaceID string
	Title       string
	Value       string
}

// SidebarDeleteDialogSnapshot describes the sidebar delete confirmation dialog.
type SidebarDeleteDialogSnapshot struct {
	Open        bool
	Kind        string
	SessionID   string
	ThreadID    string
	WorkspaceID string
	Title       string
}

// SidebarSessionGroup mirrors AppSidebar's workspace session grouping.
type SidebarSessionGroup struct {
	Key         string
	WorkspaceID string
	Label       string
	SourceType  string
	Sessions    []SidebarSessionItem
}

// SidebarSessionItem is a session row in the sidebar.
type SidebarSessionItem struct {
	ID          string
	Name        string
	DisplayName string
	Selected    bool
	Status      string
	Threads     []SidebarThreadItem
}

// SidebarThreadItem is a recent or nested thread row in the sidebar.
type SidebarThreadItem struct {
	SessionID   string
	ID          string
	Name        string
	DisplayName string
	Selected    bool
	Status      string
	State       string
	Primary     bool
	Children    []SidebarThreadItem
}

// SessionWorkspaceSnapshot is the read model for the selected session area.
type SessionWorkspaceSnapshot struct {
	Title          string
	State          string
	ThreadState    string
	Message        string
	ReserveSidebar bool
	Visible        bool
	MainClass      string
	IsPending      bool
	Composer       ConversationComposerSnapshot
	Conversation   ConversationPaneSnapshot
	Dock           DockPanelSnapshot
}

// DockPanelSnapshot describes the non-chat session dock panel area.
type DockPanelSnapshot struct {
	ActiveKind                    string
	MountedKinds                  []string
	SessionID                     string
	DockMaximized                 bool
	ShiftWindowControlsForSidebar bool
	DesktopAvailable              bool
	EditorEnabled                 bool
	VSCodeAvailable               bool
	ActiveServiceID               string
	VisibleServices               []DockService
	FileCount                     int
	DiffFileCount                 int
	Desktop                       DesktopPanelSnapshot
	DiffReview                    DiffReviewPanelSnapshot
	Files                         FilesPanelSnapshot
	Services                      ServicePanelSnapshot
	Terminal                      TerminalPanelSnapshot
	VSCode                        VSCodePanelSnapshot
}

// VSCodePanelSnapshot describes the built-in editor dock panel.
type VSCodePanelSnapshot struct {
	Service   DockService
	Loading   bool
	Error     string
	Theme     string
	AuthToken string
}

// TerminalPanelSnapshot describes the terminal dock panel.
type TerminalPanelSnapshot struct {
	SessionID        string
	ConnectionStatus string
	RootEnabled      bool
	SSHHost          string
	SSHPort          int
	CopiedCommand    string
	TerminalReady    bool
}

// DockService is a service row available in the services dock.
type DockService struct {
	ID        string
	Name      string
	Label     string
	Status    string
	URL       string
	URLPath   string
	HTTPPort  int
	HTTPSPort int
	Passive   bool
	ExitCode  *int
}

// ServicePanelSnapshot describes the services dock panel.
type ServicePanelSnapshot struct {
	ViewMode      string
	Viewport      string
	Error         string
	LogsConnected bool
	HasUnreadLogs bool
	LogEvents     []ServiceLogEvent
}

// ServiceLogEvent is one rendered service output event.
type ServiceLogEvent struct {
	Type        string
	DisplayText string
}

// DesktopPanelSnapshot describes the remote desktop dock panel.
type DesktopPanelSnapshot struct {
	ConnectionStatus string
	DesktopName      string
}

// FilesPanelSnapshot describes the files dock panel.
type FilesPanelSnapshot struct {
	ShowChangedOnly bool
	Refreshing      bool
	ActivePath      string
	OpenTabs        []FilesPanelTab
	Tree            []FilesPanelNode
	ActiveBuffer    FilesPanelBuffer
}

// FilesPanelTab is one open file tab in the files panel.
type FilesPanelTab struct {
	Path   string
	Status string
	Dirty  bool
}

// FilesPanelNode is one recursive file explorer row.
type FilesPanelNode struct {
	Name     string
	Path     string
	Type     string
	Status   string
	Changed  bool
	Expanded bool
	Loading  bool
	Children []FilesPanelNode
}

// FilesPanelBuffer is the active file content and edit state.
type FilesPanelBuffer struct {
	Content     string
	Encoding    string
	FromBase    bool
	Dirty       bool
	Saving      bool
	SaveError   string
	HasConflict bool
}

// DiffReviewFileSnapshot describes one file rendered in a diff review.
type DiffReviewFileSnapshot struct {
	Path        string
	OldPath     string
	Status      string
	Additions   int
	Deletions   int
	Binary      bool
	Patch       string
	CommitHash  string
	DiffStyle   string
	Virtualized bool
	Rendering   bool
	Expanded    bool
	Approved    bool
	Loading     bool
	Error       string
	LineCount   int
}

// DiffReviewPanelSnapshot describes the diff-review dock panel.
type DiffReviewPanelSnapshot struct {
	DiffTarget       string
	DiffTargetDraft  string
	DiffStyle        string
	Refreshing       bool
	ApprovalsLoading bool
	ApprovedCount    int
	Additions        int
	Deletions        int
	Files            []DiffReviewFileSnapshot
	SelectedComment  DiffReviewSelectionCommentSnapshot
}

// DiffReviewSelectionCommentSnapshot describes the selected diff text comment popover.
type DiffReviewSelectionCommentSnapshot struct {
	Path       string
	Text       string
	Draft      string
	Error      string
	Submitting bool
	Top        int
	Left       int
}

// ConversationPaneSnapshot is the read model for the scrollable conversation.
type ConversationPaneSnapshot struct {
	Status           string
	SessionError     string
	ThreadError      string
	ChatWidthMode    string
	Messages         []ConversationMessage
	ShowComposer     bool
	SelectionComment ConversationSelectionCommentSnapshot
}

// ConversationMessage is a simplified chat message row.
type ConversationMessage struct {
	ID            string
	Role          string
	Content       string
	Branches      []string
	CurrentBranch int
}

// MessageResponseWithCommandSnapshot describes a user message that may have an
// original slash command plus generated text parts.
type MessageResponseWithCommandSnapshot struct {
	MessageID              string
	OriginalText           string
	OriginalCommand        OriginalCommandSnapshot
	TextParts              []MessageTextPart
	GeneratedTextExpanded  bool
	GeneratedToggleCommand string
}

// OriginalCommandSnapshot is the original command display metadata.
type OriginalCommandSnapshot struct {
	Kind          string
	Command       string
	Args          string
	Text          string
	SuppressedLLM bool
}

// MessageTextPart is one text part rendered for generated message text.
type MessageTextPart struct {
	Text string
}

// ConversationSelectionCommentSnapshot describes the floating selection comment affordance.
type ConversationSelectionCommentSnapshot struct {
	Enabled    bool
	Pending    bool
	Open       bool
	Snippet    string
	Left       int
	Top        int
	Draft      string
	Submitting bool
	Error      string
}

// ConversationComposerSnapshot is the read model for the prompt composer.
type ConversationComposerSnapshot struct {
	Draft             string
	Placeholder       string
	DisabledMessage   string
	Error             string
	IsPending         bool
	IsStreaming       bool
	SubmitStatus      string
	ModelID           string
	ModelSelectionSet bool
	ModelLabel        string
	Models            []ModelOption
	ReasoningValue    string
	ReasoningSet      bool
	DefaultReasoning  string
	ReasoningLabel    string
	ReasoningLevels   []string
	ServiceTierValue  string
	ServiceTierSet    bool
	ServiceTierLabel  string
	ServiceTiers      []string
	ScheduledRunAfter string
	ScheduleOpen      bool
	Attachments       []ComposerAttachment
	QueueExpanded     bool
	PlanEntries       []PlanEntry
	PromptQueue       []QueuedPrompt
	SetupStatus       ComposerSessionSetupStatusSnapshot
	Credentials       ConversationCredentialsSnapshot
	HooksPanel        ConversationHooksPanelSnapshot
	FileMentions      FileMentionDropdownSnapshot
	SlashCommands     SlashCommandDropdownSnapshot
	PromptHistory     ConversationPromptHistorySnapshot
	WorkspaceSelector ConversationWorkspaceSelectorSnapshot
}

// FileMentionDropdownSnapshot describes @file autocomplete suggestions.
type FileMentionDropdownSnapshot struct {
	Open          bool
	Loading       bool
	Query         string
	SelectedIndex int
	Suggestions   []FileMentionItem
}

// FileMentionItem is one @file autocomplete result.
type FileMentionItem struct {
	Path string
	Type string
}

// SlashCommandDropdownSnapshot describes slash-command autocomplete suggestions.
type SlashCommandDropdownSnapshot struct {
	Open          bool
	SessionID     string
	Loading       bool
	Query         string
	SelectedIndex int
	Commands      []SlashCommand
}

// SlashCommand is one command suggestion in composer autocomplete.
type SlashCommand struct {
	Name        string
	Description string
	Order       int
}

// ComposerAttachment is one file staged in the prompt composer.
type ComposerAttachment struct {
	ID        string
	Filename  string
	MediaType string
	URL       string
	Size      int64
}

// PlanEntry is one queued or completed plan/prompt item.
type PlanEntry struct {
	ID      string
	Title   string
	Content string
	Status  string
}

// QueuedPrompt is one prompt waiting to run in the conversation queue.
type QueuedPrompt struct {
	ID              string
	Text            string
	Model           string
	RunAfter        string
	RunAfterLabel   string
	AttachmentCount int
	Saving          bool
	Editing         bool
	ScheduleOpen    bool
}

// ConversationWorkspaceSelectorSnapshot describes pending workspace setup inputs.
type ConversationWorkspaceSelectorSnapshot struct {
	FullWidth               bool
	Loading                 bool
	RequiresInput           bool
	SourceType              string
	SourceInput             string
	SelectedOption          string
	Validating              bool
	ValidationPath          string
	ValidationSourceType    string
	ValidationValid         bool
	ValidationError         string
	SetupMessage            string
	Workspaces              []WorkspaceOption
	Suggestions             []WorkspaceSuggestion
	HasSuggestionSelection  bool
	SelectedSuggestionIndex int
	Branch                  string
	Branches                []string
	ShowBranchSelector      bool
}

// CredentialsManagerSnapshot describes the project credentials settings panel.
type CredentialsManagerSnapshot struct {
	Loading          bool
	Error            string
	Credentials      []ConfiguredCredential
	ProviderGroups   []CredentialProviderGroup
	EditorOpen       bool
	EditorMode       string
	SelectedProvider string
	EnvVarRows       []CredentialEnvVarRow
	OAuthScopes      CredentialOAuthScopePickerSnapshot
	OAuthWizard      CredentialOAuthWizardSnapshot
}

// ConfiguredCredential is one configured credential in settings.
type ConfiguredCredential struct {
	ID          string
	Name        string
	TypeLabel   string
	Summary     string
	Monogram    string
	ImageSrc    string
	ImageClass  string
	Inactive    bool
	Toggling    bool
	Deleting    bool
	Visibility  CredentialVisibility
	AuthType    string
	Scopes      []string
	EnvKeys     []string
	Description string
}

// CredentialProviderGroup groups credential provider options in the picker.
type CredentialProviderGroup struct {
	Name    string
	Options []CredentialProviderOption
}

// CredentialProviderOption is one selectable credential type.
type CredentialProviderOption struct {
	ID          string
	Label       string
	Description string
	Monogram    string
	ImageSrc    string
	ImageClass  string
	AuthType    string
}

// CredentialEnvVarRow is one row in the custom environment-variable credential editor.
type CredentialEnvVarRow struct {
	ID             string
	Key            string
	Value          string
	HasStoredValue bool
	ReplaceValue   bool
	ValueFocused   bool
}

// CredentialOAuthScopePickerSnapshot describes an OAuth scope chooser.
type CredentialOAuthScopePickerSnapshot struct {
	Label            string
	Mode             string
	UseBulletSummary bool
	CanResetDefaults bool
	SimpleOptions    []CredentialOAuthScopeOption
	DefaultOptions   []CredentialOAuthScopeOption
	AdvancedGroups   []CredentialOAuthScopeGroup
}

// CredentialOAuthScopeGroup groups advanced OAuth scope options.
type CredentialOAuthScopeGroup struct {
	Group  string
	Scopes []CredentialOAuthScopeOption
}

// CredentialOAuthScopeOption is one selectable OAuth scope.
type CredentialOAuthScopeOption struct {
	Value          string
	Label          string
	SimpleLabel    string
	Description    string
	SimpleHelpText string
	Access         string
	Enabled        bool
}

// CredentialOAuthWizardSnapshot describes the OAuth connection wizard dialog.
type CredentialOAuthWizardSnapshot struct {
	Open                     bool
	Title                    string
	ProviderName             string
	OpenVerificationLabel    string
	WaitingForProviderLabel  string
	DeviceIntroLine1         string
	DeviceIntroLine2         string
	DeviceReturnText         string
	CloseLabel               string
	SelectedOAuthKind        string
	HasScopeOptions          bool
	StartingOAuth            bool
	PollingOAuth             bool
	CopiedOAuthCode          bool
	CopiedOAuthAuthURL       bool
	OAuthAuthURL             string
	OAuthInputDraft          string
	OAuthVerifierDraft       string
	OAuthVerificationURL     string
	OAuthUserCodeDraft       string
	ErrorMessage             string
	OAuthScopePickerSnapshot CredentialOAuthScopePickerSnapshot
}

// SandboxProvidersManagerSnapshot describes sandbox provider settings.
type SandboxProvidersManagerSnapshot struct {
	Loading              bool
	Saving               bool
	Error                string
	DefaultProviderID    string
	ProjectDefaultID     string
	DriverPickerOpen     bool
	FormOpen             bool
	RuntimeProviderID    string
	Providers            []SandboxProviderInstance
	ProviderTypes        []SandboxProviderType
	SelectedProviderType SandboxProviderType
}

// SandboxProviderInstance is one configured sandbox provider.
type SandboxProviderInstance struct {
	ID           string
	Type         string
	Name         string
	Icon         string
	BuiltIn      bool
	Disabled     bool
	Available    bool
	Description  string
	Capabilities SandboxProviderCapabilities
}

// SandboxProviderType is one available sandbox provider driver.
type SandboxProviderType struct {
	ID           string
	Name         string
	Icon         string
	Description  string
	Available    bool
	ConfigFields []SandboxProviderConfigField
}

// SandboxProviderConfigField describes one provider configuration input.
type SandboxProviderConfigField struct {
	Key         string
	Label       string
	Type        string
	Placeholder string
	Description string
	Required    bool
	Advanced    bool
}

// SandboxProviderCapabilities describes provider control affordances.
type SandboxProviderCapabilities struct {
	Resources  bool
	Inspection bool
}

// WorkspaceOption is an existing workspace option.
type WorkspaceOption struct {
	ID         string
	Label      string
	SourceType string
	GitHub     bool
}

// WorkspaceSuggestion is an autocomplete suggestion for workspace source input.
type WorkspaceSuggestion struct {
	Value string
	Valid bool
}

// ConversationPromptHistorySnapshot describes prompt history suggestions.
type ConversationPromptHistorySnapshot struct {
	Open            bool
	HasSelection    bool
	SelectionPinned bool
	SelectedIndex   int
	PinnedPrompts   []string
	RecentPrompts   []string
}

// ComposerSessionSetupStatusSnapshot describes pending session setup status.
type ComposerSessionSetupStatusSnapshot struct {
	SessionStatus     string
	ErrorMessage      string
	PendingStarted    bool
	WorkspacesLoading bool
	SetupMessage      string
	ValidationMessage string
	ValidationIsValid bool
	AuthMessage       string
	AuthRequired      bool
	AuthProvider      string
}

// ConversationCredentialsSnapshot describes session credential visibility.
type ConversationCredentialsSnapshot struct {
	Loading     bool
	Assignments []SessionCredentialAssignment
}

// SessionCredentialAssignment is one credential assigned to a session.
type SessionCredentialAssignment struct {
	ID         string
	Name       string
	Inactive   bool
	Visibility CredentialVisibility
	Uses       []CredentialUse
}

// CredentialVisibility indicates which runtimes may see a credential.
type CredentialVisibility struct {
	Tools    bool
	Console  bool
	Services bool
	Hooks    bool
}

// CredentialUse is an approved time-scoped credential use.
type CredentialUse struct {
	ID          string
	Description string
	Timing      string
	Expired     bool
}

// ConversationHooksPanelSnapshot describes hook status rows above the composer.
type ConversationHooksPanelSnapshot struct {
	Expanded bool
	Hooks    []HookStatus
}

// HookStatus is one hook row and optional output preview.
type HookStatus struct {
	ID             string
	Name           string
	Type           string
	DisplayState   string
	RunCount       int
	FailCount      int
	LastRunLabel   string
	LastExitCode   *int
	Command        string
	Output         string
	TooLarge       bool
	DisplayedBytes int64
	SizeBytes      int64
}
