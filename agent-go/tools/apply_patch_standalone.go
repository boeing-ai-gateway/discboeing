package tools

import (
	"fmt"
	"os"

	"github.com/boeing-ai-gateway/discboeing/agent-go/message"
)

// StandaloneApplyPatch executes apply_patch in a fresh process context.
// It primes read-before-write state for files referenced by the patch so an
// external apply_patch command can reuse the built-in tool implementation.
func StandaloneApplyPatch(cwd, dataDir, threadsDir, patch string) (string, error) {
	exec := New(cwd, dataDir, "")
	if threadsDir != "" {
		exec.SetThreadsDir(threadsDir)
	}
	if err := primeStandaloneApplyPatchReads(exec, cwd, patch); err != nil {
		return "", err
	}

	result, err := exec.executeApplyPatchText(message.ToolCallPart{
		ToolCallID: "standalone-apply-patch",
		ToolName:   "apply_patch",
	}, patch, cwd)
	if err != nil {
		return "", err
	}

	switch out := result.Result.Output.(type) {
	case message.TextOutput:
		return out.Value, nil
	case message.ErrorTextOutput:
		return "", fmt.Errorf("%s", out.Value)
	default:
		return "", fmt.Errorf("unexpected apply_patch output type %T", result.Result.Output)
	}
}

func primeStandaloneApplyPatchReads(exec *Executor, cwd, patch string) error {
	ops, err := parseApplyPatch(patch)
	if err != nil {
		return err
	}

	for _, op := range ops {
		if err := recordStandaloneApplyPatchRead(exec, resolvePath(cwd, op.path)); err != nil {
			return err
		}
		if op.kind == patchUpdateFile && op.movePath != "" {
			if err := recordStandaloneApplyPatchRead(exec, resolvePath(cwd, op.movePath)); err != nil {
				return err
			}
		}
	}
	return nil
}

func recordStandaloneApplyPatchRead(exec *Executor, path string) error {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	exec.recordFileRead(path, info)
	return nil
}
