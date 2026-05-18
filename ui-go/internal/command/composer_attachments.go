package command

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

const composerAttachmentMaxBytes = 32 << 20

// ComposerAttachments stages browser-selected files in the session-owned
// composer view. This mirrors the Svelte composer's client-side staging, but
// stores the data URLs on the server so Datastar patches can re-render chips
// without losing the selected files.
func (h *Handler) ComposerAttachments(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, composerAttachmentMaxBytes)
	if err := r.ParseMultipartForm(composerAttachmentMaxBytes); err != nil {
		h.logger.Warn("failed to parse composer attachments", "error", err)
		http.Error(w, "invalid attachments", http.StatusBadRequest)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	files := r.MultipartForm.File["attachments"]
	if len(files) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	attachments := make([]viewmodel.ComposerAttachment, 0, len(files))
	for _, fileHeader := range files {
		file, err := fileHeader.Open()
		if err != nil {
			h.logger.Warn("failed to open composer attachment", "filename", fileHeader.Filename, "error", err)
			http.Error(w, "failed to read attachment", http.StatusBadRequest)
			return
		}
		content, err := io.ReadAll(file)
		if closeErr := file.Close(); closeErr != nil {
			h.logger.Warn("failed to close composer attachment", "filename", fileHeader.Filename, "error", closeErr)
		}
		if err != nil {
			h.logger.Warn("failed to read composer attachment", "filename", fileHeader.Filename, "error", err)
			http.Error(w, "failed to read attachment", http.StatusBadRequest)
			return
		}
		id, err := newAttachmentID()
		if err != nil {
			h.logger.Warn("failed to create composer attachment id", "error", err)
			http.Error(w, "failed to stage attachment", http.StatusInternalServerError)
			return
		}
		mediaType := fileHeader.Header.Get("Content-Type")
		if mediaType == "" || mediaType == "application/octet-stream" {
			mediaType = mime.TypeByExtension(strings.ToLower(filepath.Ext(fileHeader.Filename)))
		}
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		attachments = append(attachments, viewmodel.ComposerAttachment{
			ID:        id,
			Filename:  filepath.Base(fileHeader.Filename),
			MediaType: mediaType,
			URL:       fmt.Sprintf("data:%s;base64,%s", mediaType, base64.StdEncoding.EncodeToString(content)),
			Size:      int64(len(content)),
		})
	}

	draft := r.FormValue("prompt")
	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.Draft = draft
		view.Workspace.Composer.Attachments = append(view.Workspace.Composer.Attachments, attachments...)
	})
	w.WriteHeader(http.StatusNoContent)
}

// ComposerAttachmentRemove removes one staged file from the composer.
func (h *Handler) ComposerAttachmentRemove(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.logger.Warn("failed to parse composer attachment remove form", "error", err)
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	session, ok := h.session(r)
	if !ok {
		http.Error(w, "missing session", http.StatusBadRequest)
		return
	}

	id := r.FormValue("id")
	draft := r.FormValue("prompt")
	session.Save(func(view *viewmodel.ShellSnapshot) {
		view.Workspace.Composer.Draft = draft
		if id == "" {
			return
		}
		attachments := view.Workspace.Composer.Attachments[:0]
		for _, attachment := range view.Workspace.Composer.Attachments {
			if attachment.ID != id {
				attachments = append(attachments, attachment)
			}
		}
		view.Workspace.Composer.Attachments = attachments
	})
	w.WriteHeader(http.StatusNoContent)
}

func newAttachmentID() (string, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return "attachment-" + hex.EncodeToString(b[:]), nil
}
