package command

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/obot-platform/discobot/discobot/internal/state"
)

const composerAttachmentMaxUploadBytes = 25 * 1024 * 1024

// ComposerAttachmentAdd stores pending composer attachments uploaded by the
// browser file picker or drop target.
func (h *Handler) ComposerAttachmentAdd(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, composerAttachmentMaxUploadBytes)
	if err := r.ParseMultipartForm(composerAttachmentMaxUploadBytes); err != nil {
		http.Error(w, "invalid attachment upload", http.StatusBadRequest)
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		writeNoContent(w)
		return
	}

	attachments := make([]state.ComposerAttachment, 0, len(files))
	for _, header := range files {
		file, err := header.Open()
		if err != nil {
			http.Error(w, "failed to open attachment", http.StatusBadRequest)
			return
		}
		content, err := io.ReadAll(file)
		closeErr := file.Close()
		if err != nil || closeErr != nil {
			http.Error(w, "failed to read attachment", http.StatusBadRequest)
			return
		}

		mediaType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if mediaType == "" {
			mediaType = "application/octet-stream"
		}
		attachments = append(attachments, state.ComposerAttachment{
			ID:        newComposerAttachmentID(),
			Filename:  header.Filename,
			MediaType: mediaType,
			URL:       "data:" + mediaType + ";base64," + base64.StdEncoding.EncodeToString(content),
		})
	}

	h.view.SaveView(r.Context(), func(view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		composer.Attachments = append(composer.Attachments, attachments...)
	})
	writeNoContent(w)
}

// ComposerAttachmentRemove removes a pending composer attachment.
func (h *Handler) ComposerAttachmentRemove(w http.ResponseWriter, r *http.Request) {
	attachmentID := chi.URLParam(r, "attachmentID")
	h.view.SaveView(r.Context(), func(view *state.View) {
		composer := state.EnsureComposerPanelState(view)
		for index, attachment := range composer.Attachments {
			if attachment.ID == attachmentID {
				composer.Attachments = append(composer.Attachments[:index], composer.Attachments[index+1:]...)
				break
			}
		}
	})
	writeNoContent(w)
}

func newComposerAttachmentID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "attachment"
	}
	return "attachment-" + hex.EncodeToString(bytes[:])
}
