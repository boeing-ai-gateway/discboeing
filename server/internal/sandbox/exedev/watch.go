package exedev

import (
	"context"
	"time"

	"github.com/obot-platform/discobot/server/internal/sandbox"
)

func (p *Provider) Watch(ctx context.Context) (<-chan sandbox.StateEvent, error) {
	ch := make(chan sandbox.StateEvent, 32)
	go func() {
		defer close(ch)
		last := map[string]sandbox.Status{}
		sendSnapshot := func() bool {
			sandboxes, err := p.List(ctx)
			if err != nil {
				return true
			}
			for _, sb := range sandboxes {
				if last[sb.SessionID] == sb.Status {
					continue
				}
				last[sb.SessionID] = sb.Status
				select {
				case ch <- sandbox.StateEvent{SessionID: sb.SessionID, Status: sb.Status, Timestamp: time.Now(), Error: sb.Error}:
				case <-ctx.Done():
					return false
				}
			}
			return true
		}
		if !sendSnapshot() {
			return
		}
		ticker := time.NewTicker(p.timings.watchPollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if !sendSnapshot() {
					return
				}
			case <-ctx.Done():
				return
			}
		}
	}()
	return ch, nil
}
