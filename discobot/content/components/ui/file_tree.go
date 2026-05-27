package ui

import (
	"strconv"
	"strings"
)

type FileTreeData struct {
	Label          string
	Search         string
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
	FileTreeIconSetCodicons FileTreeIconSet = "codicons"
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

func fileTreeClass(tree FileTreeData) string {
	class := "file-tree"
	density := tree.Density
	if density == "" {
		density = FileTreeDensityCompact
	}
	class += " file-tree--" + string(density)
	if tree.DragEnabled && strings.TrimSpace(tree.Search) == "" {
		class += " file-tree--drag-enabled"
	}
	if tree.LargeTreeLimit > 0 && tree.TotalNodeCount > tree.LargeTreeLimit {
		class += " file-tree--large"
	}
	return class
}

func fileTreeNodeClass(node FileTreeNode) string {
	class := "file-tree__node"
	switch node.Kind {
	case FileTreeNodeDirectory:
		class += " file-tree__node--directory"
	case FileTreeNodeFile:
		class += " file-tree__node--file"
	}
	if node.Kind == FileTreeNodeDirectory && node.Expanded {
		class += " file-tree__node--expanded"
	}
	if node.Selected {
		class += " file-tree__node--selected"
	}
	if node.GitStatus != "" && node.GitStatus != FileTreeGitStatusClean {
		class += " file-tree__node--status-" + string(node.GitStatus)
	}
	if node.HasChangedDescendants {
		class += " file-tree__node--has-changed-descendants"
	}
	if node.CanDrop {
		class += " file-tree__node--drop-target"
	}
	return class
}

func fileTreeNodePadding(depth int) string {
	return "padding-left: calc(2px + " + fileTreeDepth(depth) + " * var(--file-tree-indent))"
}

func fileTreeDepth(depth int) string {
	if depth <= 0 {
		return "0"
	}
	return strconv.Itoa(depth)
}

func fileTreeCommandMethod(command FileTreeCommand) string {
	if command.Method == "" {
		return "POST"
	}
	return command.Method
}

func fileTreeCommandAction(command FileTreeCommand) string {
	return fileTreeDiscobotCommandAction(command)
}

func fileTreeNodeClickAction(node FileTreeNode) string {
	if node.Kind == FileTreeNodeDirectory && node.HasChildren {
		return fileTreeCommandAction(node.ToggleCommand)
	}
	return fileTreeCommandAction(node.SelectCommand)
}

func fileTreeDiscobotCommandAction(command FileTreeCommand) string {
	if command.URL == "" {
		return ""
	}
	return "@discobotCommand(" + strconv.Quote(command.URL) + ", {method: " + strconv.Quote(fileTreeCommandMethod(command)) + "})"
}

func fileTreeSearchAction(command FileTreeCommand) string {
	if command.URL == "" {
		return ""
	}
	return "@discobotCommand(" + strconv.Quote(command.URL) + ", {method: " + strconv.Quote(fileTreeCommandMethod(command)) + ", payload: {query: evt.target.value}})"
}

func fileTreeDeleteAction(command FileTreeCommand) string {
	if command.URL == "" {
		return ""
	}
	return "if (confirm('Delete this file tree item and its descendants?')) " + fileTreeDiscobotCommandAction(command)
}

func fileTreeRenameAction(node FileTreeNode) string {
	if node.RenameCommand.URL == "" {
		return ""
	}
	return "const name = prompt('Rename item', " + strconv.Quote(node.Name) + "); if (name && name.trim()) @discobotCommand(" + strconv.Quote(node.RenameCommand.URL) + ", {method: " + strconv.Quote(fileTreeCommandMethod(node.RenameCommand)) + ", payload: {name: name.trim()}})"
}

func fileTreeCreateAction(command FileTreeCommand, kind string) string {
	if command.URL == "" {
		return ""
	}
	return "const name = prompt('New " + kind + " name'); if (name && name.trim()) @discobotCommand(" + strconv.Quote(command.URL) + ", {method: " + strconv.Quote(fileTreeCommandMethod(command)) + ", payload: {name: name.trim()}})"
}

func fileTreeStatusLabel(status FileTreeGitStatus) string {
	switch status {
	case FileTreeGitStatusModified:
		return "M"
	case FileTreeGitStatusAdded:
		return "A"
	case FileTreeGitStatusDeleted:
		return "D"
	case FileTreeGitStatusRenamed:
		return "R"
	case FileTreeGitStatusUntracked:
		return "U"
	case FileTreeGitStatusIgnored:
		return "I"
	default:
		return ""
	}
}

func fileTreeStatusTitle(status FileTreeGitStatus) string {
	label := string(status)
	if label == "" || status == FileTreeGitStatusClean {
		return ""
	}
	return "Git status: " + label
}

func fileTreeMovePayloadAction(command FileTreeCommand) string {
	if command.URL == "" {
		return ""
	}
	return "@discobotCommand(" + strconv.Quote(command.URL) + ", {method: " + strconv.Quote(fileTreeCommandMethod(command)) + ", payload: {targetID: evt.detail?.targetID || ''}})"
}

func fileTreeSearchActive(search string) string {
	if strings.TrimSpace(search) == "" {
		return "false"
	}
	return "true"
}

func fileTreeTriggerMode(tree FileTreeData) string {
	if tree.TriggerMode == "" {
		return string(FileTreeTriggerBoth)
	}
	return string(tree.TriggerMode)
}

func fileTreeShowMenuTrigger(tree FileTreeData) bool {
	return tree.TriggerMode == "" || tree.TriggerMode == FileTreeTriggerButton || tree.TriggerMode == FileTreeTriggerBoth
}

func fileTreeNodeIcon(node FileTreeNode) string {
	if node.Icon != "" {
		return node.Icon
	}
	if node.Kind == FileTreeNodeDirectory {
		return ""
	}
	name := strings.ToLower(node.Name)
	switch {
	case strings.HasSuffix(name, ".md") || strings.HasSuffix(name, ".markdown"):
		return "markdown"
	case strings.HasSuffix(name, ".json") || strings.HasSuffix(name, ".jsonc"):
		return "json"
	case strings.HasSuffix(name, ".sh") || strings.HasSuffix(name, ".bash") || strings.HasSuffix(name, ".ps1"):
		return "terminal"
	case strings.HasSuffix(name, ".png") || strings.HasSuffix(name, ".jpg") || strings.HasSuffix(name, ".jpeg") || strings.HasSuffix(name, ".gif") || strings.HasSuffix(name, ".svg"):
		return "file-media"
	case strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".sum"):
		return "symbol-key"
	case strings.HasSuffix(name, ".go") || strings.HasSuffix(name, ".js") || strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx") || strings.HasSuffix(name, ".css") || strings.HasSuffix(name, ".templ"):
		return "file-code"
	default:
		return "file"
	}
}

func fileTreeNodeIconClass(node FileTreeNode) string {
	class := "icon-xs"
	if node.IconColorClass != "" {
		class += " " + node.IconColorClass
	}
	return class
}

func fileTreeCanDrag(tree FileTreeData, node FileTreeNode) bool {
	return tree.DragEnabled && strings.TrimSpace(tree.Search) == "" && node.CanDrag && node.MoveCommand.URL != ""
}

func fileTreeCanRootDrop(tree FileTreeData) bool {
	return tree.DragEnabled && strings.TrimSpace(tree.Search) == "" && tree.Controls.RootDropCommand.URL != ""
}

func fileTreeLargeNotice(tree FileTreeData) string {
	if tree.LargeTreeLimit <= 0 || tree.TotalNodeCount <= tree.LargeTreeLimit {
		return ""
	}
	return "Showing " + strconv.Itoa(tree.RenderedCount) + " of " + strconv.Itoa(tree.TotalNodeCount) + " visible file rows"
}

func fileTreeHighlightedName(name string, search string) (string, string, string, bool) {
	query := strings.TrimSpace(search)
	if query == "" {
		return name, "", "", false
	}
	index := strings.Index(strings.ToLower(name), strings.ToLower(query))
	if index < 0 {
		return name, "", "", false
	}
	return name[:index], name[index : index+len(query)], name[index+len(query):], true
}

func fileTreeMenu(node FileTreeNode) MenuData {
	items := []MenuItem{}
	if node.Kind == FileTreeNodeDirectory {
		items = append(items,
			MenuItem{Label: "New File", Action: fileTreeCreateAction(node.NewFileCommand, "file")},
			MenuItem{Label: "New Folder", Action: fileTreeCreateAction(node.NewFolderCommand, "folder")},
		)
	}
	items = append(items,
		MenuItem{Label: "Rename", SeparatorBefore: len(items) > 0, Action: fileTreeRenameAction(node)},
		MenuItem{Label: "Delete", SeparatorBefore: true, Action: fileTreeDeleteAction(node.DeleteCommand)},
	)
	return MenuData{
		ID:      "file-tree-menu-" + node.ID,
		Label:   "Open actions for " + node.Name,
		Trigger: Codicon("kebab-vertical", "icon-xs"),
		Items:   items,
	}
}

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func fileTreeIndexString(index int) string {
	return strconv.Itoa(index)
}
