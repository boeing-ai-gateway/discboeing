package state

import serverapi "github.com/obot-platform/discobot/server/api"

// View is server-owned UI view state.
type View struct {
	GlobalPanelLayout   GlobalPanelLayout
	SessionPanelLayouts map[string]*SessionPanelLayout
	Settings            SettingsDialogState
}

// SettingsDialogState owns server-rendered settings dialog state.
type SettingsDialogState struct {
	Open                 bool
	SupportInfoOpen      bool
	ClearCacheDialogOpen bool
	CacheCleared         bool
	Tab                  SettingsTab
}

// SettingsTab describes the active settings section.
type SettingsTab string

const (
	// SettingsTabAppearance shows mode and palette preferences.
	SettingsTabAppearance SettingsTab = "appearance"
	// SettingsTabChat shows conversation defaults.
	SettingsTabChat SettingsTab = "chat"
	// SettingsTabProviders shows sandbox provider settings.
	SettingsTabProviders SettingsTab = "providers"
	// SettingsTabUpdate shows update and cache tools.
	SettingsTabUpdate SettingsTab = "update"
	// SettingsTabCredentials shows API credential settings.
	SettingsTabCredentials SettingsTab = "credentials"
)

// NormalizeSettingsTab returns a supported settings tab.
func NormalizeSettingsTab(tab SettingsTab) SettingsTab {
	switch tab {
	case SettingsTabAppearance, SettingsTabChat, SettingsTabProviders, SettingsTabUpdate, SettingsTabCredentials:
		return tab
	default:
		return SettingsTabAppearance
	}
}

// GlobalPanelLayout is the server-owned app-wide panel layout.
type GlobalPanelLayout struct {
	SessionSidebar Panel[SessionSidebarState]
}

// SessionPanelLayout is the server-owned per-session workspace panel layout.
type SessionPanelLayout struct {
	Editor       Panel[EditorPanelState]
	Conversation Panel[ConversationPanelState]
	Terminal     Panel[ConversationTerminalState]
}

// Panel describes server-owned panel sizing bounds, current size, and state.
type Panel[T any] struct {
	ID          string
	Visible     bool
	Maximizable bool
	Maximized   bool
	GridColumn  string
	GridRow     string
	Width       int
	Height      int
	MinWidth    int
	MaxWidth    int
	MinHeight   int
	MaxHeight   int
	State       T
}

// PanelFrame contains panel layout data without the panel-specific state.
type PanelFrame = Panel[struct{}]

// SessionSidebarState owns state for the global session navigation/file-tree panel.
type SessionSidebarState struct {
	SelectedSessionID            string
	SelectedFileID               string
	SelectedSideChatID           string
	ExpandedSessionIDs           map[string]bool
	VisibleSessionDetailSections map[string]bool
	ExpandedFileIDs              map[string]bool
	SessionViewModes             map[string]SessionViewMode
	SessionMenuChecks            map[string]bool
	FileTreeSearch               string
	FileTreeSearchVisible        bool
}

// SessionPanelState is kept as an alias for session sidebar state.
type SessionPanelState = SessionSidebarState

// SessionViewMode describes the expanded session item content.
type SessionViewMode string

const (
	// SessionViewModeFiles shows the session file tree.
	SessionViewModeFiles SessionViewMode = "files"
	// SessionViewModeDiff shows a flat changed-file list.
	SessionViewModeDiff SessionViewMode = "diff"
)

// NormalizeSessionViewMode returns a supported session view mode.
func NormalizeSessionViewMode(mode SessionViewMode) SessionViewMode {
	if mode == SessionViewModeDiff {
		return SessionViewModeDiff
	}
	return SessionViewModeFiles
}

// SessionDetailSection describes a visible panel in an expanded session row.
type SessionDetailSection string

const (
	// SessionDetailSectionWorkspace shows the session file/diff workspace.
	SessionDetailSectionWorkspace SessionDetailSection = "workspace"
	// SessionDetailSectionSideChats shows compact side-chat thread rows.
	SessionDetailSectionSideChats SessionDetailSection = "side-chats"
	// SessionDetailSectionHooks shows hook run state.
	SessionDetailSectionHooks SessionDetailSection = "hooks"
	// SessionDetailSectionReview shows diff review and approval controls.
	SessionDetailSectionReview SessionDetailSection = "review"
)

// IsSessionDetailSection reports whether section is supported.
func IsSessionDetailSection(section SessionDetailSection) bool {
	switch section {
	case SessionDetailSectionWorkspace, SessionDetailSectionSideChats, SessionDetailSectionHooks, SessionDetailSectionReview:
		return true
	default:
		return false
	}
}

// SessionDetailSectionKey scopes a section visibility state to a session.
func SessionDetailSectionKey(sessionID string, section SessionDetailSection) string {
	return sessionID + ":" + string(section)
}

// EditorPanelState owns state for an editor panel instance.
type EditorPanelState struct {
	OpenFileIDs          []string
	ActiveFileID         string
	DiffSummarySessionID string
	ServiceLogID         string
}

// ComposerAttachment stores a pending composer file attachment.
type ComposerAttachment struct {
	ID        string
	Filename  string
	MediaType string
	URL       string
}

// ConversationPanelState owns state for a session conversation/composer panel instance.
type ConversationPanelState struct {
	SelectedWorkspaceID    string
	SelectedModelID        string
	SelectedReasoning      string
	SelectedServiceTier    string
	Attachments            []ComposerAttachment
	WorkspaceSourceType    string
	WorkspaceSourceInput   string
	WorkspaceValidation    serverapi.ValidateWorkspaceResponse
	WorkspaceValidationSet bool
	WorkspaceSetupMessage  string
}

// ComposerPanelState is kept as an alias for conversation panel state.
type ComposerPanelState = ConversationPanelState

// ConversationTerminalState owns state for a session terminal panel instance.
type ConversationTerminalState struct{}

// TerminalPanelState is kept as an alias for conversation terminal state.
type TerminalPanelState = ConversationTerminalState

// WorkspacePanelIDs returns the mutually visible workspace panels.
func WorkspacePanelIDs() []string {
	return []string{"editor", "composer", "terminal"}
}

// IsWorkspacePanel reports whether a panel participates in the main workspace.
func IsWorkspacePanel(panelID string) bool {
	for _, id := range WorkspacePanelIDs() {
		if id == panelID {
			return true
		}
	}
	return false
}

// PanelVisible reports a panel's effective visibility, including defaults.
func PanelVisible(view View, panelID string) bool {
	panel, ok := PanelFrameForSession(view, view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID, panelID)
	return ok && panel.Visible
}

// VisibleWorkspacePanelCount returns the number of visible workspace panels.
func VisibleWorkspacePanelCount(view View) int {
	count := 0
	for _, panelID := range WorkspacePanelIDs() {
		if PanelVisible(view, panelID) {
			count++
		}
	}
	return count
}

// CanHideWorkspacePanel prevents the workspace from having no visible panels.
func CanHideWorkspacePanel(view View, panelID string) bool {
	if !IsWorkspacePanel(panelID) || !PanelVisible(view, panelID) {
		return true
	}
	return VisibleWorkspacePanelCount(view) > 1
}

// PanelFrameForSession returns layout data for a known panel.
func PanelFrameForSession(view View, sessionID string, panelID string) (PanelFrame, bool) {
	view = NormalizeView(view)
	if sessionID == "" {
		sessionID = view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	}
	layout := view.SessionPanelLayouts[sessionID]
	if layout == nil {
		defaultLayout := DefaultSessionPanelLayout()
		layout = &defaultLayout
	}
	switch panelID {
	case "session":
		return panelFrame(view.GlobalPanelLayout.SessionSidebar), true
	case "editor":
		return panelFrame(layout.Editor), true
	case "composer":
		return panelFrame(layout.Conversation), true
	case "terminal":
		return panelFrame(layout.Terminal), true
	default:
		return PanelFrame{}, false
	}
}

func panelFrame[T any](panel Panel[T]) PanelFrame {
	return PanelFrame{
		ID:          panel.ID,
		Visible:     panel.Visible,
		Maximizable: panel.Maximizable,
		Maximized:   panel.Maximized,
		GridColumn:  panel.GridColumn,
		GridRow:     panel.GridRow,
		Width:       panel.Width,
		Height:      panel.Height,
		MinWidth:    panel.MinWidth,
		MaxWidth:    panel.MaxWidth,
		MinHeight:   panel.MinHeight,
		MaxHeight:   panel.MaxHeight,
	}
}

// PanelFrameOf returns layout data for a typed panel.
func PanelFrameOf[T any](panel Panel[T]) PanelFrame {
	return panelFrame(panel)
}

func applyPanelFrame[T any](panel Panel[T], frame PanelFrame) Panel[T] {
	panel.ID = frame.ID
	panel.Visible = frame.Visible
	panel.Maximizable = frame.Maximizable
	panel.Maximized = frame.Maximized
	panel.GridColumn = frame.GridColumn
	panel.GridRow = frame.GridRow
	panel.Width = frame.Width
	panel.Height = frame.Height
	panel.MinWidth = frame.MinWidth
	panel.MaxWidth = frame.MaxWidth
	panel.MinHeight = frame.MinHeight
	panel.MaxHeight = frame.MaxHeight
	return panel
}

// DefaultView returns the initial server-owned view state.
func DefaultView() View {
	return View{
		GlobalPanelLayout: DefaultGlobalPanelLayout(),
		SessionPanelLayouts: map[string]*SessionPanelLayout{
			"session-cobra": new(DefaultSessionPanelLayout()),
		},
		Settings: SettingsDialogState{
			Tab: SettingsTabAppearance,
		},
	}
}

// DefaultGlobalPanelLayout returns the initial app-wide panel layout.
func DefaultGlobalPanelLayout() GlobalPanelLayout {
	return GlobalPanelLayout{
		SessionSidebar: Panel[SessionSidebarState]{
			ID:          "session",
			Visible:     true,
			Maximizable: false,
			GridColumn:  "1",
			GridRow:     "1 / -1",
			Width:       280,
			MinWidth:    220,
			MaxWidth:    460,
			State: SessionSidebarState{
				SelectedSessionID:  "session-cobra",
				SelectedSideChatID: "thread-cobra-review",
				ExpandedSessionIDs: map[string]bool{
					"session-cobra": true,
				},
				ExpandedFileIDs: map[string]bool{
					"file-cobra-root":                  true,
					"file-cobra-agent":                 true,
					"file-cobra-agent-provider":        true,
					"file-cobra-agent-provider-openai": true,
					"file-cobra-vm":                    true,
					"file-cobra-vm-network":            true,
					"file-cobra-vm-scripts":            true,
					"file-cobra-cmd":                   true,
					"file-cobra-cmd-discobot":          true,
					"file-cobra-internal":              true,
					"file-cobra-internal-command":      true,
					"file-cobra-internal-state":        true,
					"file-cobra-internal-server":       true,
					"file-cobra-content":               true,
					"file-cobra-content-components":    true,
					"file-cobra-content-app":           true,
					"file-cobra-content-ui":            true,
					"file-cobra-static":                true,
					"file-cobra-static-lib":            true,
					"file-cobra-styles":                true,
					"file-cobra-docs":                  true,
					"file-cobra-docs-design":           true,
					"file-cobra-tests":                 true,
					"file-cobra-tests-fixtures":        true,
					"file-cobra-scripts":               true,
				},
				SessionMenuChecks: map[string]bool{
					"copilot-cli":          true,
					"cloud":                true,
					"claude":               true,
					"completed":            true,
					"in-progress":          true,
					"input-needed":         true,
					"failed":               true,
					"done":                 false,
					"read":                 true,
					"sort-created":         true,
					"sort-updated":         false,
					"group-workspace":      true,
					"group-time":           false,
					"show-recent-sessions": true,
					"show-all-sessions":    false,
				},
			},
		},
	}
}

// DefaultSessionPanelLayout returns the initial per-session panel layout.
func DefaultSessionPanelLayout() SessionPanelLayout {
	return SessionPanelLayout{
		Editor: Panel[EditorPanelState]{
			ID:          "editor",
			Visible:     true,
			Maximizable: true,
			GridColumn:  "5 / 6",
			GridRow:     "1",
			State: EditorPanelState{
				OpenFileIDs: []string{},
			},
		},
		Conversation: Panel[ConversationPanelState]{
			ID:          "composer",
			Visible:     true,
			Maximizable: true,
			GridColumn:  "3",
			GridRow:     "1",
			Width:       360,
			MinWidth:    280,
			MaxWidth:    560,
			State:       ConversationPanelState{},
		},
		Terminal: Panel[ConversationTerminalState]{
			ID:          "terminal",
			Visible:     true,
			Maximizable: true,
			GridColumn:  "5 / 6",
			GridRow:     "3",
			Height:      220,
			MinHeight:   160,
			MaxHeight:   420,
			State:       ConversationTerminalState{},
		},
	}
}

// DefaultPanelFrames returns state-free panel defaults keyed by panel ID.
func DefaultPanelFrames() map[string]PanelFrame {
	globalLayout := DefaultGlobalPanelLayout()
	sessionLayout := DefaultSessionPanelLayout()
	return map[string]PanelFrame{
		"session":  panelFrame(globalLayout.SessionSidebar),
		"editor":   panelFrame(sessionLayout.Editor),
		"composer": panelFrame(sessionLayout.Conversation),
		"terminal": panelFrame(sessionLayout.Terminal),
	}
}

// NormalizeView returns a cloned view with all known panel defaults applied.
func NormalizeView(view View) View {
	view = cloneView(view)
	view.Settings.Tab = NormalizeSettingsTab(view.Settings.Tab)
	view.GlobalPanelLayout.SessionSidebar = normalizePanel(
		view.GlobalPanelLayout.SessionSidebar,
		DefaultGlobalPanelLayout().SessionSidebar,
	)
	if view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID == "" {
		view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID = DefaultGlobalPanelLayout().SessionSidebar.State.SelectedSessionID
	}

	selectedSessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	if view.SessionPanelLayouts == nil {
		view.SessionPanelLayouts = map[string]*SessionPanelLayout{}
	}
	layout := DefaultSessionPanelLayout()
	if view.SessionPanelLayouts[selectedSessionID] != nil {
		layout = *view.SessionPanelLayouts[selectedSessionID]
	}
	layout = normalizeSessionPanelLayout(layout)
	view.SessionPanelLayouts[selectedSessionID] = new(layout)
	normalizeWorkspacePanelPlacement(&view)
	return view
}

func normalizeSessionPanelLayout(layout SessionPanelLayout) SessionPanelLayout {
	defaultLayout := DefaultSessionPanelLayout()
	layout.Editor = normalizePanel(layout.Editor, defaultLayout.Editor)
	layout.Conversation = normalizePanel(layout.Conversation, defaultLayout.Conversation)
	layout.Terminal = normalizePanel(layout.Terminal, defaultLayout.Terminal)
	return layout
}

func normalizePanel[T any](panel Panel[T], defaultPanel Panel[T]) Panel[T] {
	if panel.ID == "" {
		return defaultPanel
	}
	frame := normalizePanelFrame(panelFrame(panel), panelFrame(defaultPanel))
	return applyPanelFrame(panel, frame)
}

func normalizePanelFrame(panel PanelFrame, defaultPanel PanelFrame) PanelFrame {
	if panel.ID == "" {
		panel.ID = defaultPanel.ID
	}
	if defaultPanel.Maximizable {
		panel.Maximizable = true
	} else {
		panel.Maximizable = false
		panel.Maximized = false
	}
	if panel.GridColumn == "" {
		panel.GridColumn = defaultPanel.GridColumn
	}
	if panel.GridRow == "" {
		panel.GridRow = defaultPanel.GridRow
	}
	if panel.Width == 0 {
		panel.Width = defaultPanel.Width
	}
	if panel.Height == 0 {
		panel.Height = defaultPanel.Height
	}
	if panel.MinWidth == 0 {
		panel.MinWidth = defaultPanel.MinWidth
	}
	if panel.MaxWidth == 0 {
		panel.MaxWidth = defaultPanel.MaxWidth
	}
	if panel.MinHeight == 0 {
		panel.MinHeight = defaultPanel.MinHeight
	}
	if panel.MaxHeight == 0 {
		panel.MaxHeight = defaultPanel.MaxHeight
	}

	return panel
}

func normalizeShellView(data Data, view View) View {
	if len(Sessions(data)) == 0 {
		panel := view.GlobalPanelLayout.SessionSidebar
		panel.Visible = false
		view.GlobalPanelLayout.SessionSidebar = panel
		normalizeWorkspacePanelPlacement(&view)
	}
	return view
}

func normalizeWorkspacePanelPlacement(view *View) {
	sessionVisible := view.GlobalPanelLayout.SessionSidebar.Visible
	workspaceColumns := "3 / 6"
	workspaceStartColumn := "3"
	if !sessionVisible {
		workspaceColumns = "1 / 6"
		workspaceStartColumn = "1"
	}

	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	layout := view.SessionPanelLayouts[sessionID]
	if layout == nil {
		defaultLayout := DefaultSessionPanelLayout()
		layout = &defaultLayout
		view.SessionPanelLayouts[sessionID] = layout
	}
	editor := layout.Editor
	composer := layout.Conversation
	terminal := layout.Terminal
	editorVisible := editor.Visible
	composerVisible := composer.Visible
	terminalVisible := terminal.Visible

	switch {
	case editorVisible && composerVisible && terminalVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		editor.GridColumn = "5 / 6"
		editor.GridRow = "1"
		terminal.GridColumn = "5 / 6"
		terminal.GridRow = "3"
	case editorVisible && composerVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		editor.GridColumn = "5 / 6"
		editor.GridRow = "1 / -1"
	case editorVisible && terminalVisible:
		editor.GridColumn = workspaceColumns
		editor.GridRow = "1"
		terminal.GridColumn = workspaceColumns
		terminal.GridRow = "3"
	case composerVisible && terminalVisible:
		composer.GridColumn = workspaceStartColumn + " / 4"
		composer.GridRow = "1 / -1"
		terminal.GridColumn = "5 / 6"
		terminal.GridRow = "1 / -1"
	case editorVisible:
		editor.GridColumn = workspaceColumns
		editor.GridRow = "1 / -1"
	case composerVisible:
		composer.GridColumn = workspaceColumns
		composer.GridRow = "1 / -1"
	case terminalVisible:
		terminal.GridColumn = workspaceColumns
		terminal.GridRow = "1 / -1"
	}

	layout.Editor = editor
	layout.Conversation = composer
	layout.Terminal = terminal
}

// EnsurePanel returns panel layout data, initializing it from defaults when needed.
func EnsurePanel(view *View, panelID string) PanelFrame {
	*view = NormalizeView(*view)
	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	panel, _ := PanelFrameForSession(*view, sessionID, panelID)
	return panel
}

// SavePanel writes panel layout data back to the correct global or session layout.
func SavePanel(view *View, panelID string, panel PanelFrame) {
	*view = NormalizeView(*view)
	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	switch panelID {
	case "session":
		view.GlobalPanelLayout.SessionSidebar = applyPanelFrame(view.GlobalPanelLayout.SessionSidebar, panel)
	case "editor":
		layout := view.SessionPanelLayouts[sessionID]
		layout.Editor = applyPanelFrame(layout.Editor, panel)
	case "composer":
		layout := view.SessionPanelLayouts[sessionID]
		layout.Conversation = applyPanelFrame(layout.Conversation, panel)
	case "terminal":
		layout := view.SessionPanelLayouts[sessionID]
		layout.Terminal = applyPanelFrame(layout.Terminal, panel)
	}
	normalizeWorkspacePanelPlacement(view)
}

// EnsureSessionPanelState returns mutable state for the global session sidebar.
func EnsureSessionPanelState(view *View) *SessionPanelState {
	*view = NormalizeView(*view)
	return &view.GlobalPanelLayout.SessionSidebar.State
}

// EnsureEditorPanelState returns mutable state for the editor panel.
func EnsureEditorPanelState(view *View) *EditorPanelState {
	*view = NormalizeView(*view)
	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	return &view.SessionPanelLayouts[sessionID].Editor.State
}

// EnsureComposerPanelState returns mutable state for the composer panel.
func EnsureComposerPanelState(view *View) *ComposerPanelState {
	*view = NormalizeView(*view)
	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	return &view.SessionPanelLayouts[sessionID].Conversation.State
}

// EnsureTerminalPanelState returns mutable state for the terminal panel.
func EnsureTerminalPanelState(view *View) *TerminalPanelState {
	*view = NormalizeView(*view)
	sessionID := view.GlobalPanelLayout.SessionSidebar.State.SelectedSessionID
	return &view.SessionPanelLayouts[sessionID].Terminal.State
}

func cloneView(view View) View {
	view.GlobalPanelLayout.SessionSidebar.State = *cloneSessionPanelState(&view.GlobalPanelLayout.SessionSidebar.State)
	if view.SessionPanelLayouts != nil {
		layouts := make(map[string]*SessionPanelLayout, len(view.SessionPanelLayouts))
		for key, value := range view.SessionPanelLayouts {
			if value == nil {
				continue
			}
			value := *value
			if editor := cloneEditorPanelState(&value.Editor.State); editor != nil {
				value.Editor.State = *editor
			}
			if composer := cloneComposerPanelState(&value.Conversation.State); composer != nil {
				value.Conversation.State = *composer
			}
			layouts[key] = &value
		}
		view.SessionPanelLayouts = layouts
	}
	return view
}

func cloneSessionPanelState(state *SessionPanelState) *SessionPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	clone.ExpandedSessionIDs = cloneBoolMap(state.ExpandedSessionIDs)
	clone.VisibleSessionDetailSections = cloneBoolMap(state.VisibleSessionDetailSections)
	clone.ExpandedFileIDs = cloneBoolMap(state.ExpandedFileIDs)
	clone.SessionViewModes = cloneSessionViewModeMap(state.SessionViewModes)
	clone.SessionMenuChecks = cloneBoolMap(state.SessionMenuChecks)
	return &clone
}

func cloneEditorPanelState(state *EditorPanelState) *EditorPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.OpenFileIDs != nil {
		clone.OpenFileIDs = append([]string(nil), state.OpenFileIDs...)
	}
	return &clone
}

func cloneComposerPanelState(state *ComposerPanelState) *ComposerPanelState {
	if state == nil {
		return nil
	}
	clone := *state
	if state.Attachments != nil {
		clone.Attachments = append([]ComposerAttachment(nil), state.Attachments...)
	}
	return &clone
}

func cloneBoolMap(source map[string]bool) map[string]bool {
	if source == nil {
		return nil
	}
	clone := make(map[string]bool, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}

func cloneSessionViewModeMap(source map[string]SessionViewMode) map[string]SessionViewMode {
	if source == nil {
		return nil
	}
	clone := make(map[string]SessionViewMode, len(source))
	for key, value := range source {
		clone[key] = value
	}
	return clone
}
