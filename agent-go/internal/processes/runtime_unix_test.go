//go:build !windows

package processes

import (
	"reflect"
	"testing"
)

func TestSudoCommandForUserUsesGateWithTargetUser(t *testing.T) {
	got := sudoCommandForUser("root", []string{"/bin/sh", "-lc", "id -u"})
	want := []string{sudoPath, "-E", "-n", "-u", "root", "--", "/bin/sh", "-lc", "id -u"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sudoCommandForUser() = %#v, want %#v", got, want)
	}
}

func TestSudoCommandForUserSupportsUIDAndGID(t *testing.T) {
	got := sudoCommandForUser("1000:1001", []string{"id"})
	want := []string{sudoPath, "-E", "-n", "-u", "#1000", "-g", "#1001", "--", "id"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("sudoCommandForUser() = %#v, want %#v", got, want)
	}
}

func TestCommandForUserLeavesDefaultUserUnwrapped(t *testing.T) {
	cmd := []string{"/bin/echo", "ok"}
	got, err := commandForUser(cmd, "")
	if err != nil {
		t.Fatalf("commandForUser() failed: %v", err)
	}
	if !reflect.DeepEqual(got, cmd) {
		t.Fatalf("commandForUser() = %#v, want %#v", got, cmd)
	}
}

func TestCommandForUserWrapsDifferentUserWithSudo(t *testing.T) {
	cmd := []string{"/bin/echo", "ok"}
	got, err := commandForUser(cmd, "discobot-test-user-that-should-not-exist")
	if err != nil {
		t.Fatalf("commandForUser() failed: %v", err)
	}
	want := []string{sudoPath, "-E", "-n", "-u", "discobot-test-user-that-should-not-exist", "--", "/bin/echo", "ok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("commandForUser() = %#v, want %#v", got, want)
	}
}
