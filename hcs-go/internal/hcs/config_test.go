package hcs

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/obot-platform/discobot/hcs-go/internal/cli"
)

func TestBuildKernelCommandLineNoInitrd(t *testing.T) {
	options := cli.DefaultOptions()
	options.InitrdPath = nil
	options.RootDevice = "/dev/sda"
	options.ProcessorCount = 4
	got := BuildKernelCommandLine(options)
	if strings.Contains(got, "initrd=") {
		t.Fatalf("command line unexpectedly contains initrd: %s", got)
	}
	for _, want := range []string{"root=/dev/sda", "rootfstype=ext4", "nr_cpus=4", "console=hvc0"} {
		if !strings.Contains(got, want) {
			t.Fatalf("command line %q missing %q", got, want)
		}
	}
}

func TestBuildConfigurationOmitsNilInitrd(t *testing.T) {
	options := cli.DefaultOptions()
	options.VMID = uuid.MustParse("aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee")
	options.InitrdPath = nil
	options.RootDiskPath = `C:\vm\root.vhdx`
	options.DataDiskPath = `C:\vm\data.vhdx`
	text, err := BuildConfiguration(options)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(text), &doc); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(text, "InitRdPath") {
		t.Fatalf("configuration should omit nil InitRdPath: %s", text)
	}
	if !strings.Contains(text, `"ShouldTerminateOnLastHandleClosed": true`) {
		t.Fatalf("configuration missing lifetime setting: %s", text)
	}
}

func TestBuildPlan9ShareAdd(t *testing.T) {
	text, err := BuildPlan9ShareAdd(cli.Plan9Share{Name: "src", HostPath: `C:\src`, ReadOnly: true})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`"RequestType": "Add"`, `"Name": "src"`, `"Flags": 5`} {
		if !strings.Contains(text, want) {
			t.Fatalf("Plan9 JSON %q missing %q", text, want)
		}
	}
}
