package handler

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/files"
	"github.com/obot-platform/discobot/agent-go/internal/gitops"
)

const defaultSessionStreamPingInterval = 15 * time.Second

// StreamSession handles GET /session/stream. It streams agent-owned session
// resource snapshots. A fresh connection starts with history-start, sends a
// complete resource snapshot, then history-end, and remains open for live
// session-level resource changes.
func (h *Handler) StreamSession(w http.ResponseWriter, r *http.Request) {
	if !h.requireConversations(w) {
		return
	}
	resources := sessionStreamResourcesFromRequest(r)
	flusher, ok := w.(http.Flusher)
	if !ok {
		h.Error(w, http.StatusInternalServerError, "streaming not supported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	writeEvent := func(event string, payload any) bool {
		var data []byte
		if payload == nil {
			data = json.RawMessage(`{}`)
		} else {
			var err error
			data, err = json.Marshal(payload)
			if err != nil {
				log.Printf("session stream: failed to marshal %s: %v", event, err)
				return false
			}
		}
		writeSSEEvent(w, "", event, data)
		flusher.Flush()
		return true
	}

	if !writeEvent("history-start", nil) {
		return
	}
	if !h.writeSessionResourceSnapshots(writeEvent, resources) {
		return
	}
	if !writeEvent("history-end", nil) {
		return
	}

	var changes <-chan struct{} = make(chan struct{})
	unsubscribe := func() {}
	if h.activity != nil {
		changes, unsubscribe = h.activity.Subscribe()
	}
	defer unsubscribe()

	var fileChanges <-chan struct{} = make(chan struct{})
	stopFileWatcher := func() {}
	if resources.includesWorkspaceState() {
		fileChanges, stopFileWatcher = h.subscribeWorkspaceFileChanges()
	}
	defer stopFileWatcher()

	pingEvery := h.chatPingEvery
	if pingEvery <= 0 {
		pingEvery = defaultSessionStreamPingInterval
	}
	pingTicker := time.NewTicker(pingEvery)
	defer pingTicker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case <-changes:
			drainSessionStreamChanges(changes)
			if !h.writeAgentStateSnapshots(writeEvent, resources) {
				return
			}
		case <-fileChanges:
			drainSessionStreamChanges(fileChanges)
			if !h.writeWorkspaceStateSnapshots(writeEvent, resources) {
				return
			}
		case <-pingTicker.C:
			if !writeEvent("ping", nil) {
				return
			}
		}
	}
}

type sessionStreamResources map[string]bool

func sessionStreamResourcesFromRequest(r *http.Request) sessionStreamResources {
	raw := strings.TrimSpace(r.URL.Query().Get("resources"))
	if raw == "" {
		return nil
	}
	resources := sessionStreamResources{}
	for part := range strings.SplitSeq(raw, ",") {
		resource := strings.TrimSpace(part)
		if resource != "" {
			resources[resource] = true
		}
	}
	return resources
}

func (r sessionStreamResources) includes(resource string) bool {
	return len(r) == 0 || r[resource]
}

func (r sessionStreamResources) includesWorkspaceState() bool {
	return r.includes("files") || r.includes("diff") || r.includes("status")
}

func drainSessionStreamChanges(changes <-chan struct{}) {
	for {
		select {
		case <-changes:
			continue
		default:
			return
		}
	}
}

func (h *Handler) writeSessionResourceSnapshots(writeEvent func(string, any) bool, resources sessionStreamResources) bool {
	if !h.writeAgentStateSnapshots(writeEvent, resources) {
		return false
	}
	return h.writeWorkspaceStateSnapshots(writeEvent, resources)
}

func (h *Handler) writeAgentStateSnapshots(writeEvent func(string, any) bool, resources sessionStreamResources) bool {
	if resources.includes("threads") {
		threads, err := h.threadsSnapshot()
		if err != nil {
			writeEvent("error", api.ErrorResponse{Error: err.Error()})
			return false
		}
		if !writeEvent("threads_updated", threads) {
			return false
		}
	}

	if resources.includes("commands") {
		commands, err := h.commandsSnapshot()
		if err != nil {
			writeEvent("error", api.ErrorResponse{Error: err.Error()})
			return false
		}
		if !writeEvent("commands_updated", commands) {
			return false
		}
	}

	if resources.includes("hooks") {
		if !writeEvent("hooks_updated", h.hooksStateSnapshot()) {
			return false
		}
	}

	if resources.includes("services") {
		services, err := h.servicesSnapshot()
		if err != nil {
			writeEvent("error", api.ErrorResponse{Error: err.Error()})
			return false
		}
		if !writeEvent("services_updated", services) {
			return false
		}
	}
	return true
}

func (h *Handler) writeWorkspaceStateSnapshots(writeEvent func(string, any) bool, resources sessionStreamResources) bool {
	if resources.includes("files") {
		fileSnapshot, fileErr := files.ListDirectory(".", h.agentCwd, false)
		if fileErr != nil {
			writeEvent("error", api.ErrorResponse{Error: fileErr.Message})
			return false
		}
		if !writeEvent("files_updated", api.ListFilesResponse{
			Path:    fileSnapshot.Path,
			Entries: fileSnapshot.Entries,
		}) {
			return false
		}
	}

	if resources.includes("diff") || resources.includes("status") {
		diff, err := gitops.GetDiff(h.agentCwd, "", "")
		if err != nil {
			writeEvent("error", api.ErrorResponse{Error: err.Error()})
			return false
		}
		if resources.includes("status") && !writeEvent("diff_status_updated", diffFilesSnapshot(diff)) {
			return false
		}
		if resources.includes("diff") && !writeEvent("diff_updated", diffSnapshot(diff)) {
			return false
		}
	}
	return true
}

func (h *Handler) threadsSnapshot() (api.ListThreadsResponse, error) {
	infos, err := h.threadManager.ListThreadInfos()
	if err != nil {
		return api.ListThreadsResponse{}, err
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

	threads := make([]api.Thread, 0, len(infos))
	for _, info := range infos {
		threads = append(threads, h.threadResponse(info))
	}
	return api.ListThreadsResponse{Threads: threads}, nil
}

func (h *Handler) commandsSnapshot() (api.ListCommandsResponse, error) {
	commands, err := h.conversations.ListCommands()
	if err != nil {
		return api.ListCommandsResponse{}, err
	}

	sort.SliceStable(commands, func(i, j int) bool {
		if commands[i].Discobot.Order != commands[j].Discobot.Order {
			return commands[i].Discobot.Order < commands[j].Discobot.Order
		}
		return commands[i].Name < commands[j].Name
	})
	return api.ListCommandsResponse{Commands: commands}, nil
}

func (h *Handler) hooksStateSnapshot() api.HooksStateResponse {
	status := h.hooksStatusResponse()
	resp := api.HooksStateResponse{
		HooksStatusResponse: status,
		Outputs:             map[string]api.HookOutputResponse{},
	}
	if h.hookManager == nil {
		return resp
	}
	for hookID := range status.Hooks {
		output, err := h.hookManager.GetHookOutput(hookID)
		if err != nil {
			continue
		}
		resp.Outputs[hookID] = api.HookOutputResponse{
			Output:         output.Output,
			SizeBytes:      output.SizeBytes,
			DisplayedBytes: output.DisplayedBytes,
			TooLarge:       output.TooLarge,
		}
	}
	return resp
}

func (h *Handler) servicesSnapshot() (api.ListServicesResponse, error) {
	if h.serviceManager == nil {
		return api.ListServicesResponse{Services: []api.Service{}}, nil
	}
	svcList, err := h.serviceManager.GetServices(h.agentCwd)
	if err != nil {
		return api.ListServicesResponse{}, err
	}

	apiServices := make([]api.Service, len(svcList))
	for i, svc := range svcList {
		apiServices[i] = toAPIService(svc)
	}
	return api.ListServicesResponse{Services: apiServices}, nil
}

func diffFilesSnapshot(diff gitops.DiffResult) api.DiffFilesResponse {
	fileEntries := make([]api.DiffFileEntry, len(diff.Files))
	for i, file := range diff.Files {
		fileEntries[i] = api.DiffFileEntry{
			Path:    file.Path,
			Status:  file.Status,
			OldPath: file.OldPath,
		}
	}
	return api.DiffFilesResponse{
		Files: fileEntries,
		Stats: diff.Stats,
	}
}

func diffSnapshot(diff gitops.DiffResult) api.DiffResponse {
	return api.DiffResponse(diff)
}

func (h *Handler) subscribeWorkspaceFileChanges() (<-chan struct{}, func()) {
	if h.workspaceFiles == nil {
		h.workspaceFiles = newWorkspaceFileNotifier(h.agentCwd)
	}
	return h.workspaceFiles.Subscribe()
}
