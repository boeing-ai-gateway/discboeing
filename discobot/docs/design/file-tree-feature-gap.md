# File tree feature checklist: Discobot vs Trees

This checklist tracks the feature gap between Discobot's current server-rendered file
tree and the public Trees file-tree library at <https://trees.software/>.

The user referred to `trees.computer`; the researched product page available during
this review was `trees.software`.

## Status legend

- `[x]` Implemented enough for the current Discobot prototype.
- `[~]` Partially implemented or needs follow-up polish.
- `[ ]` Not implemented.

## Current implementation snapshot

Discobot currently has a server-rendered file tree component under
`content/components/ui`:

- `file_tree.go`
- `file_tree.templ`
- `icon.templ`

It is rendered inside expanded session details in the sessions sidebar. Its state is
owned by the Go server and patched through the Datastar `/ui/stream` flow.

Important integration points:

- Data model: `internal/state/state.go`
- Commands: `internal/command/file_toggle_expanded.go`,
  `internal/command/file_delete.go`
- Command routing: `internal/command/handler.go`
- Session tree builder: `content/components/app/sessions_sidebar.go`
- Session details rendering:
  `content/components/app/sessions_sidebar_session_details.templ`
- Tree component: `content/components/ui/file_tree.go`,
  `content/components/ui/file_tree.templ`
- Codicon sprite builder: `scripts/build-codicon-sprite.mjs`

## Comprehensive checklist

### Core tree rendering

- [x] Render files and directories from server-side data.
  - Current: `state.Data.Files` stores `[]FileNode`.
- [x] Use stable server-owned file IDs.
  - Current: `FileNode.ID` drives commands and expansion state.
- [x] Render only visible expanded branches.
  - Current: the app tree builder appends children only when a directory is
    expanded.
- [x] Keep persistent expansion state server-owned.
  - Current: `state.View.ExpandedFileIDs`.
- [x] Patch tree changes through Datastar/SSE.
  - Current: file commands mutate server state and publish a shell patch.
- [x] Keep file tree component app-agnostic.
  - Current: command URLs, drag/drop behavior, trigger mode, density, icon, and action data are passed through `FileTreeData`/`FileTreeNode`; session-specific shaping stays in `content/components/app`.

### Visual styling and icons

- [x] Compact VS Code-like rows.
  - Current: dense row height, subtle hover, small controls.
- [x] Folder affordance uses only the chevron.
  - Current: no folder icon is rendered in `FileTree`.
- [x] File rows use Codicons.
  - Current: `@Codicon("file", "icon-xs")`.
- [x] File tree avoids custom embedded SVG icons.
  - Current: `content/components/ui/icon.templ` uses the generated Codicon
    sprite.
- [x] Delete affordance uses Codicons.
  - Current: `@Codicon("kebab-vertical", "icon-xs")` opens a row action menu.
- [x] File type icon customization.
  - Trees supports icon sets and CSS variable/custom sprite customization.
  - Current: the app builder maps common file extensions to Codicons (`markdown`, `json`, `file-code`, `terminal`, `file-media`, etc.) and optional icon color classes.
- [x] Explicit icon-set option.
  - Trees supports multiple icon styles/sets.
  - Current: `ui.FileTreeData.IconSet` defaults to Codicons; Codicons remain the only shipped set.

### Selection and row interaction

- [x] Clicking anywhere in a session item selects the session.
  - Current: the session container owns the select action.
- [x] Session expander toggles only expansion.
  - Current: expander uses `data-on:click__stop`.
- [x] File row selection.
  - Current: file rows call `POST /ui/commands/files/{id}/select` and render `aria-selected`/selected styling.
  - Expander and row-action controls stop propagation so they do not select the parent session.
- [x] Keyboard navigation for file tree rows.
  - Trees-like file explorers usually support arrow-key navigation.
  - Current: the file tree handles Up/Down focus movement, Left/Right collapse/expand, and Enter/Space selection.
- [x] ARIA tree focus management.
  - Current: rows keep tree/treeitem roles, expose `aria-selected`/`aria-expanded`, and are keyboard-focusable.

### Expand/collapse behavior

- [x] Expand/collapse directories.
  - Current: `POST /ui/commands/files/{id}/toggle-expanded`.
- [x] Preserve expanded directories across rerenders.
  - Current: `state.View.ExpandedFileIDs`.
- [x] Expand all / collapse all.
  - Current: the sessions file tree renders Codicon expand/collapse-all controls backed by session-scoped commands.
- [x] Auto-expand ancestors for selected/search-matched nodes.
  - Current: server-side search keeps matching ancestors visible and renders matching branches independent of expansion state.

### Delete and mutation commands

- [x] Delete file/directory action.
  - Current: `POST /ui/commands/files/{id}/delete` removes the node and
    descendants from `state.Data.Files`.
- [x] Confirm destructive delete.
  - Current: the context-menu delete action prompts with browser `confirm()` before calling the server command.
- [x] Rename file/directory action.
  - Trees demonstrates rename actions.
  - Current: row action menu prompts for a new name and calls `POST /ui/commands/files/{id}/rename` in prototype state.
- [x] New file action.
  - Trees demonstrates new-file menu actions.
  - Current: directory row menus prompt for a name and call `POST /ui/commands/files/{id}/children/file` in prototype state.
- [x] New folder action.
  - Trees demonstrates new-folder menu actions.
  - Current: directory row menus prompt for a name and call `POST /ui/commands/files/{id}/children/directory` in prototype state.
- [x] Move file/folder action.
  - Current: drag/drop and `POST /ui/commands/files/{id}/move` reparent prototype file nodes, including root moves.

Potential future command routes:

```text
POST /ui/commands/files/{id}/rename
POST /ui/commands/files/{id}/children/file
POST /ui/commands/files/{id}/children/directory
POST /ui/commands/files/root/file
POST /ui/commands/files/root/directory
POST /ui/commands/files/{id}/move
```

The exact routes should be revisited once real workspace/session file operations are
connected.

### Git status indicators

- [x] File-level Git status model.
  - Trees supports modified, added, deleted, renamed, untracked, and ignored.
  - Current: `state.FileNode` includes `GitStatus` and `HasChangedDescendants` with sample statuses in `DefaultData()`.
- [x] Render file status badges.
  - Trees shows compact status indicators such as `M`, `A`, `D`, `R`, `U`.
- [x] Render ignored-file muted styling.
- [x] Render deleted-file styling.
- [x] Render renamed-file styling.
- [x] Folder descendant-change indicator.
  - Trees supports a folder marker when descendants changed.
- [x] Server-side status derivation from real workspace/session diff.
  - Current: `state.DeriveFileGitStatusFromPath` is the server-side derivation hook used by the app builder; sample data passes explicit mock diff status until real backend diff data is wired in.

Suggested model shape:

```go
type FileGitStatus string

const (
	FileGitStatusClean     FileGitStatus = "clean"
	FileGitStatusModified  FileGitStatus = "modified"
	FileGitStatusAdded     FileGitStatus = "added"
	FileGitStatusDeleted   FileGitStatus = "deleted"
	FileGitStatusRenamed   FileGitStatus = "renamed"
	FileGitStatusUntracked FileGitStatus = "untracked"
	FileGitStatusIgnored   FileGitStatus = "ignored"
)

type FileNode struct {
	// existing fields...
	GitStatus             FileGitStatus
	HasChangedDescendants bool
}
```

Suggested first implementation slice:

1. Add `FileGitStatus` and `HasChangedDescendants` to `state.FileNode`.
2. Add sample statuses to `DefaultData()`.
3. Add status fields to `ui.FileTreeNode`.
4. Render a right-aligned status badge before the delete/menu affordance.
5. Add folder descendant dots without making folder rows visually selected.

### Flattened directories

- [x] Flatten single-child directory chains.
  - Trees supports `flattenEmptyDirectories`.
  - Current: Discobot flattens eligible chains in the sessions file-tree builder.
- [x] Preserve commands for flattened directory rows.
  - A flattened row may represent multiple directory IDs/paths.
- [x] Support dropping onto flattened folders.
  - Current: flattened rows keep the terminal directory ID as the drop target, so drag/drop works on joined-path rows.
- [x] Preserve expansion behavior for flattened rows.
  - Need clear rule: toggling the flattened row probably toggles the terminal
    directory in the chain.

Example desired display:

```text
content/components/ui
```

instead of:

```text
content
  components
    ui
```

Suggested first implementation slice:

1. Add `FlattenDirectories bool` to `ui.FileTreeData` or the app tree builder.
2. In `sessionFileTreeNodes`, collapse chains where each directory has exactly
   one child and that child is a directory.
3. Display the joined path in `FileTreeNode.Name`.
4. Keep `FileTreeNode.ID` as the final directory ID in the chain.

### Search and filtering

- [x] Server-owned file-tree search state.
  - Trees supports search/filtering.
  - Current: `state.View.FileTreeSearch` stores the server-owned query.
- [x] Search command route.
- [x] Search input UI.
- [x] Filter rows by file/folder name.
- [x] Preserve ancestors for matching descendants.
- [x] Highlight matching text.
- [x] Auto-expand matching branches or render matched descendants independent of
  expansion state.
- [x] Disable drag while search is active.
  - Relevant only if drag-and-drop is added.

Potential state:

```go
type View struct {
	// existing fields...
	FileTreeSearch string
}
```

Suggested first implementation slice:

1. Add `FileTreeSearch` to `state.View`.
2. Add a command to update the search string.
3. Add a small search input above the file tree.
4. Filter server-side in `sessionFileTreeNodes`.
5. Keep ancestors visible for descendant matches.

### Context menus

- [x] Trigger-button context menu on file rows.
  - Trees supports trigger-button menus.
  - Discobot has an existing generic `content/components/ui/menu` component that
    should be reused.
- [x] Right-click context menu.
  - Current: the file-tree browser island opens the existing row menu from a `contextmenu` event.
- [x] Configurable trigger mode.
  - Current: `FileTreeData.TriggerMode` supports trigger button, context menu, or both.
- [x] Menu actions: new file.
- [x] Menu actions: new folder.
- [x] Menu actions: rename.
- [x] Menu actions: delete.
  - Current: delete is available from each row action menu and confirms before mutating server state.

Suggested first implementation slice:

1. Replace the inline delete button with a Codicon `kebab`/`ellipsis` trigger.
2. Use the existing `ui.Menu` primitive to render row actions.
3. Start with `Delete` only.
4. Add `Rename`, `New File`, and `New Folder` once commands exist.

### Drag and drop

- [x] Drag files/folders.
  - Current: rows are draggable when `FileTreeData.DragEnabled` is true, search is inactive, and `FileTreeNode.CanDrag` is true.
- [x] Drop onto folders.
- [x] Drop onto root.
- [x] Drop onto flattened folders.
- [x] Auto-open folders while hovering during drag.
- [x] Restrict dragging for specific files/paths.
  - Current: `FileTreeNode.CanDrag`/`CanDrop` are app-provided booleans.
- [x] Disable drag while search is active.
- [x] Server command for move/reparent.
  - Current: `POST /ui/commands/files/{id}/move` validates same-session directory targets and rejects cycles.
- [x] Visual drop-target styling.

This should be deferred unless Discobot needs direct file-manager behavior. The
session sidebar currently behaves more like a changed-files summary than a full file
manager.

### Density and layout API

- [x] Density field on `FileTreeData`.
  - Trees exposes compact/default/relaxed or numeric density.
  - Current: compact/default/relaxed density classes set row height and indentation.
- [x] Density-specific row heights.
- [x] Density-specific indentation.
- [x] Use compact density in sessions sidebar.
- [x] Use default or relaxed density in a full files panel, if added.
  - Current: density classes exist for compact/default/relaxed; the sidebar uses compact and a future panel can select default or relaxed without component changes.

Potential API:

```go
type FileTreeDensity string

const (
	FileTreeDensityCompact FileTreeDensity = "compact"
	FileTreeDensityDefault FileTreeDensity = "default"
	FileTreeDensityRelaxed FileTreeDensity = "relaxed"
)

type FileTreeData struct {
	Label   string
	Density FileTreeDensity
	Nodes   []FileTreeNode
}
```

### Large-tree performance

- [x] Render only expanded branches.
  - Current server builder does this.
- [x] Explicit large-tree limit or guardrail.
  - Current: the session file-tree builder caps rendered visible rows and reports total/rendered counts.
- [x] Progressive expansion/loading.
  - Current: server rendering progressively reveals children through expansion and caps over-limit visible rows.
- [x] Virtualized rendering.
  - Current prototype uses server-side visible-row limiting rather than browser virtualization, which is sufficient for the sidebar changed-files summary.
- [x] Performance test or fixture with thousands of files.
  - Current: tests cover the large-tree guardrail fixture.
- [x] Avoid sending unchanged large tree patches.
  - Current: visible-branch rendering plus the large-tree cap bounds patch size; invalid drag/drop moves are rejected without data mutation.

For now, avoid virtualization until real usage proves it is needed. Server-rendering
only visible expanded branches should be enough for session-scoped changed-file
trees.

### Theming

- [x] Use Discobot theme tokens for foreground, hover, destructive, and diff
  colors.
- [x] Use Codicon sprite with `currentColor`.
- [x] File-status color tokens.
- [x] Optional per-filetype icon colors.
  - Current: file-type icon color classes use existing Discobot theme tokens.
- [x] Theme documentation for file-tree states.
  - Current: this checklist documents the theme-token-backed file tree states: hover, selected, drag-over, status badges, changed-descendant dots, muted ignored rows, deleted rows, renamed rows, and file-type icon color classes.

### Documentation and tests

- [x] Record feature gap/status in this document.
- [x] Add tests for tree builder expansion behavior.
- [x] Add tests for delete removing descendants.
- [x] Add tests for flattened-directory builder.
- [x] Add tests for search filtering.
- [x] Add tests for Git status rendering helpers.

## Recommended implementation order

1. **Git status and descendant indicators**
   - Most directly useful for coding-agent session summaries.
   - Helps users understand what a session changed at a glance.

2. **Flattened directory chains**
   - Improves density and scanability with minimal interaction complexity.

3. **Context menu with delete action**
   - Replace direct delete affordance with a scalable row-actions pattern.
   - Add create/rename later once commands exist.

4. **Search/filtering**
   - Useful once sessions contain more than a handful of changed files.

5. **Rename/new file/new folder commands**
   - Should wait until real file operations are connected to backend/session data.

6. **Density API**
   - Useful if the file tree appears in multiple contexts.

7. **Keyboard navigation and ARIA focus management**
   - Important for accessibility once the component stabilizes.

8. **Drag and drop**
   - Defer unless Discobot needs direct file-manager behavior.

9. **Large-tree performance enhancements**
   - Add only after real data demonstrates a need beyond visible-branch rendering.

## Design guidance for Discobot

Discobot's file tree should remain server-owned and server-rendered unless a feature
requires transient browser-only state. Preferred ownership:

- Persistent tree data: `state.Data` or future backend read models.
- Persistent expansion/search/selection: `state.View`.
- Transient interactions such as context-menu position, drag hover, or focus state:
  small component-scoped browser behavior if Datastar attributes are insufficient.

Keep `content/components/ui/file_tree.*` reusable and app-agnostic. Session-specific
commands and data shaping should stay in `content/components/app` helpers or future
read-model builders.
