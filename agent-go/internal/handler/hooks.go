package handler

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/agent-go/internal/api"
	"github.com/obot-platform/discobot/agent-go/internal/hooks"
	"github.com/obot-platform/discobot/agent-go/internal/sudoauth"
)

// HooksStatus handles GET /hooks/status — returns hook evaluation status.
func (h *Handler) HooksStatus(w http.ResponseWriter, _ *http.Request) {
	h.JSON(w, http.StatusOK, h.hooksStatusResponse())
}

func (h *Handler) hooksStatusResponse() api.HooksStatusResponse {
	if h.hookManager == nil {
		return api.HooksStatusResponse{
			Hooks:           map[string]api.HookRunStatus{},
			PendingHooks:    []string{},
			LastEvaluatedAt: "",
			ExecutionPaused: false,
		}
	}

	status := h.hookManager.GetStatus()
	resp := api.HooksStatusResponse{
		Hooks:           make(map[string]api.HookRunStatus, len(status.Hooks)),
		PendingHooks:    status.PendingHooks,
		LastEvaluatedAt: status.LastEvaluatedAt,
		ExecutionPaused: status.ExecutionPaused,
	}
	if resp.PendingHooks == nil {
		resp.PendingHooks = []string{}
	}

	for id, s := range status.Hooks {
		resp.Hooks[id] = api.HookRunStatus{
			HookID:              s.HookID,
			HookName:            s.HookName,
			Type:                s.Type,
			Engine:              s.Engine,
			Phase:               s.Phase,
			LastRunAt:           s.LastRunAt,
			LastResult:          s.LastResult,
			LastExitCode:        s.LastExitCode,
			OutputPath:          s.OutputPath,
			RunCount:            s.RunCount,
			FailCount:           s.FailCount,
			ConsecutiveFailures: s.ConsecutiveFailures,
			ExecutionPaused:     s.ExecutionPaused,
		}
	}

	return resp
}

// UpdateHooksExecution handles PATCH /hooks/execution — toggles hook execution.
func (h *Handler) UpdateHooksExecution(w http.ResponseWriter, r *http.Request) {
	if h.hookManager == nil {
		h.Error(w, http.StatusNotFound, "Hooks not enabled")
		return
	}

	var req api.UpdateHooksExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.hookManager.SetExecutionPaused(req.Paused); err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, h.hooksStatusResponse())
}

// UpdateHookExecution handles PATCH /hooks/{hookId}/execution — toggles
// execution for one hook.
func (h *Handler) UpdateHookExecution(w http.ResponseWriter, r *http.Request) {
	if h.hookManager == nil {
		h.Error(w, http.StatusNotFound, "Hooks not enabled")
		return
	}

	hookID := chi.URLParam(r, "hookId")
	if hookID == "" {
		h.Error(w, http.StatusBadRequest, "hookId is required")
		return
	}

	var req api.UpdateHooksExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.Error(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := h.hookManager.SetHookExecutionPaused(hookID, req.Paused); err != nil {
		if strings.Contains(err.Error(), "not found") {
			h.Error(w, http.StatusNotFound, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, h.hooksStatusResponse())
}

// HooksState handles GET /hooks/state — returns hook status and inline outputs.
func (h *Handler) HooksState(w http.ResponseWriter, _ *http.Request) {
	status := h.hooksStatusResponse()
	resp := api.HooksStateResponse{
		HooksStatusResponse: status,
		Outputs:             map[string]api.HookOutputResponse{},
	}

	if h.hookManager == nil {
		h.JSON(w, http.StatusOK, resp)
		return
	}

	for hookID := range status.Hooks {
		output, err := h.hookManager.GetHookOutput(hookID)
		if err != nil {
			h.Error(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp.Outputs[hookID] = api.HookOutputResponse{
			Output:         output.Output,
			SizeBytes:      output.SizeBytes,
			DisplayedBytes: output.DisplayedBytes,
			TooLarge:       output.TooLarge,
		}
	}

	h.JSON(w, http.StatusOK, resp)
}

// HookOutput handles GET /hooks/{hookId}/output — returns hook output log.
func (h *Handler) HookOutput(w http.ResponseWriter, r *http.Request) {
	hookID := chi.URLParam(r, "hookId")

	if h.hookManager == nil {
		h.Error(w, http.StatusNotFound, "Hooks not enabled")
		return
	}

	output, err := h.hookManager.GetHookOutput(hookID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.JSON(w, http.StatusOK, api.HookOutputResponse{
		Output:         output.Output,
		SizeBytes:      output.SizeBytes,
		DisplayedBytes: output.DisplayedBytes,
		TooLarge:       output.TooLarge,
	})
}

// HookOutputDownload handles GET /hooks/{hookId}/output/download — returns hook output log as an attachment.
func (h *Handler) HookOutputDownload(w http.ResponseWriter, r *http.Request) {
	hookID := chi.URLParam(r, "hookId")

	if h.hookManager == nil {
		h.Error(w, http.StatusNotFound, "Hooks not enabled")
		return
	}

	output, err := h.hookManager.GetHookOutputDownload(hookID)
	if err != nil {
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", hookID+".log"))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(output); err != nil {
		log.Printf("hooks: failed to write hook output download: %v", err)
	}
}

// RerunHook handles POST /hooks/{hookId}/rerun — manually reruns a hook.
func (h *Handler) RerunHook(w http.ResponseWriter, r *http.Request) {
	hookID := chi.URLParam(r, "hookId")
	threadID := chi.URLParam(r, "id")

	if h.hookManager == nil {
		h.Error(w, http.StatusNotFound, "Hooks not enabled")
		return
	}

	if err := h.hookManager.ValidateRerunHook(hookID); err != nil {
		if errors.Is(err, hooks.ErrHookPaused) {
			h.Error(w, http.StatusConflict, err.Error())
			return
		}
		if errors.Is(err, hooks.ErrHookNotFound) {
			h.Error(w, http.StatusNotFound, err.Error())
			return
		}
		h.Error(w, http.StatusInternalServerError, err.Error())
		return
	}

	go h.withManualSessionHookSudo(func() {
		result, err := h.hookManager.RerunHook(hookID)
		if err != nil {
			log.Printf("hooks: failed to rerun hook %q: %v", hookID, err)
			return
		}
		if result == nil {
			log.Printf("hooks: rerun hook %q completed without a result", hookID)
			return
		}

		if result.Eval.ShouldReprompt {
			if err := h.hookManager.StartFailureReprompt(threadID, result.Eval); err != nil {
				log.Printf("hooks: failed to start re-prompt for manual rerun: %v", err)
			}
		}
	})

	h.JSON(w, http.StatusOK, api.HookRerunResponse{
		Success:  true,
		ExitCode: 0,
	})
}

func (h *Handler) withManualSessionHookSudo(fn func()) {
	if h.sudoAuthorizer == nil {
		fn()
		return
	}
	token, err := randomSudoBootstrapToken()
	if err != nil {
		log.Printf("hooks: failed to create manual session hook sudo token: %v", err)
		fn()
		return
	}
	h.sudoAuthorizer.RegisterBootstrapToken(token)
	defer h.sudoAuthorizer.RevokeBootstrapToken(token)
	h.hookManager.SetStartupHookEnv(func(hook hooks.Hook) map[string]string {
		return manualSessionHookSudoEnv(hook, token)
	})
	defer h.hookManager.SetStartupHookEnv(nil)
	fn()
}

func manualSessionHookSudoEnv(hook hooks.Hook, token string) map[string]string {
	if hook.RunAs != "root" {
		return nil
	}
	return map[string]string{
		sudoauth.TokenEnvVar:             token,
		"DISCOBOT_SUDO_RUNTIME":          "bootstrap",
		"DISCOBOT_SUDO_COMMAND":          hook.Path,
		"DISCOBOT_SUDO_BOOTSTRAP_REASON": "manual session hook rerun " + hook.Name,
	}
}

func randomSudoBootstrapToken() (string, error) {
	var token [32]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(token[:]), nil
}
