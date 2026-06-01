package helpers

type FileTreeData struct {
	Label          string
	Search         string
	SearchVisible  bool
	Density        FileTreeDensity
	IconSet        FileTreeIconSet
	TriggerMode    FileTreeTriggerMode
	DragEnabled    bool
	LargeTreeLimit int
	TotalNodeCount int
	RenderedCount  int
	Controls       FileTreeControls
	Nodes          []FileTreeNode
}

type FileTreeControls struct {
	SearchCommand      FileTreeCommand
	ExpandAllCommand   FileTreeCommand
	CollapseAllCommand FileTreeCommand
	RootDropCommand    FileTreeCommand
	LoadMoreCommand    FileTreeCommand
}

type FileTreeNode struct {
	ID                    string
	Name                  string
	Kind                  FileTreeNodeKind
	Depth                 int
	Expanded              bool
	Selected              bool
	HasChildren           bool
	GitStatus             FileTreeGitStatus
	HasChangedDescendants bool
	Icon                  string
	IconColorClass        string
	CanDrag               bool
	CanDrop               bool
	ToggleCommand         FileTreeCommand
	SelectCommand         FileTreeCommand
	DeleteCommand         FileTreeCommand
	RenameCommand         FileTreeCommand
	NewFileCommand        FileTreeCommand
	NewFolderCommand      FileTreeCommand
	MoveCommand           FileTreeCommand
	DropCommand           FileTreeCommand
}

type FileTreeNodeKind string

const (
	FileTreeNodeDirectory FileTreeNodeKind = "directory"
	FileTreeNodeFile      FileTreeNodeKind = "file"
)

type FileTreeDensity string

const (
	FileTreeDensityCompact FileTreeDensity = "compact"
	FileTreeDensityDefault FileTreeDensity = "default"
	FileTreeDensityRelaxed FileTreeDensity = "relaxed"
)

type FileTreeIconSet string

const (
	FileTreeIconSetLucide FileTreeIconSet = "lucide"
)

type FileTreeTriggerMode string

const (
	FileTreeTriggerButton      FileTreeTriggerMode = "button"
	FileTreeTriggerContextMenu FileTreeTriggerMode = "context-menu"
	FileTreeTriggerBoth        FileTreeTriggerMode = "both"
)

type FileTreeGitStatus string

const (
	FileTreeGitStatusClean     FileTreeGitStatus = "clean"
	FileTreeGitStatusModified  FileTreeGitStatus = "modified"
	FileTreeGitStatusAdded     FileTreeGitStatus = "added"
	FileTreeGitStatusDeleted   FileTreeGitStatus = "deleted"
	FileTreeGitStatusRenamed   FileTreeGitStatus = "renamed"
	FileTreeGitStatusUntracked FileTreeGitStatus = "untracked"
	FileTreeGitStatusIgnored   FileTreeGitStatus = "ignored"
)

type FileTreeCommand struct {
	Method string
	URL    string
}
