package server

import (
	"encoding/json"
	"log"
	"time"

	"github.com/obot-platform/discobot/agent-go/filewatcher"
	"github.com/obot-platform/discobot/agent-go/message"
)

const workspaceFilesDataType = "workspace-files"

type workspaceFileEvent struct {
	Root     string               `json:"root"`
	Changes  []filewatcher.Change `json:"changes"`
	Resync   bool                 `json:"resync,omitempty"`
	Snapshot []filewatcher.Entry  `json:"snapshot,omitempty"`
	Error    *workspaceFileError  `json:"error,omitempty"`
}

type workspaceFileError struct {
	Message string `json:"message"`
}

type workspaceFileWatcher struct {
	watcher *filewatcher.Watcher
	done    chan struct{}
}

func startWorkspaceFileWatcher(workspaceRoot string, emit func(message.MessageChunk)) (*workspaceFileWatcher, error) {
	watcher, err := filewatcher.New(workspaceRoot, filewatcher.Options{
		IncludeInitial: true,
		ResyncInterval: 30 * time.Second,
	})
	if err != nil {
		return nil, err
	}

	w := &workspaceFileWatcher{
		watcher: watcher,
		done:    make(chan struct{}),
	}
	go w.run(emit)
	return w, nil
}

func (w *workspaceFileWatcher) Close() error {
	err := w.watcher.Close()
	<-w.done
	return err
}

func (w *workspaceFileWatcher) run(emit func(message.MessageChunk)) {
	defer close(w.done)
	for w.watcher != nil {
		select {
		case batch, ok := <-w.watcher.Events():
			if !ok {
				return
			}
			emitWorkspaceFileEvent(emit, workspaceFileEvent{
				Root:     batch.Root,
				Changes:  batch.Changes,
				Resync:   batch.Resync,
				Snapshot: batch.Snapshot,
			})
		case err, ok := <-w.watcher.Errors():
			if !ok {
				return
			}
			if err == nil {
				continue
			}
			log.Printf("workspace file watcher: %v", err)
			emitWorkspaceFileEvent(emit, workspaceFileEvent{
				Root: w.watcher.Root(),
				Error: &workspaceFileError{
					Message: err.Error(),
				},
			})
		}
	}
}

func emitWorkspaceFileEvent(emit func(message.MessageChunk), event workspaceFileEvent) {
	if emit == nil {
		return
	}
	data, err := json.Marshal(event)
	if err != nil {
		log.Printf("workspace file watcher: marshal event: %v", err)
		return
	}
	emit(message.DataChunk{
		DataType: workspaceFilesDataType,
		Data:     data,
	})
}
