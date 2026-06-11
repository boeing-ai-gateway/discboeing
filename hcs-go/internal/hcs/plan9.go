package hcs

import "github.com/obot-platform/discobot/hcs-go/internal/cli"

const (
	plan9ShareFlagsReadOnly      = 0x00000001
	plan9ShareFlagsLinuxMetadata = 0x00000004
)

func BuildPlan9ShareAdd(share cli.Plan9Share) (string, error) {
	return buildPlan9Share("Add", plan9ShareSettings(share, true))
}

func BuildPlan9ShareRemove(share cli.Plan9Share) (string, error) {
	return buildPlan9Share("Remove", plan9ShareSettings(share, false))
}

func buildPlan9Share(requestType string, settings map[string]any) (string, error) {
	document := map[string]any{
		"ResourcePath": "VirtualMachine/Devices/Plan9/Shares",
		"RequestType":  requestType,
		"Settings":     settings,
	}
	return marshalIndented(document)
}

func plan9ShareSettings(share cli.Plan9Share, includePath bool) map[string]any {
	settings := map[string]any{
		"Name":       share.Name,
		"AccessName": share.Name,
		"Port":       cli.Plan9DefaultPort,
	}
	if includePath {
		settings["Path"] = share.HostPath
		settings["Flags"] = plan9ShareFlags(share)
	}
	return settings
}

func plan9ShareFlags(share cli.Plan9Share) int {
	flags := plan9ShareFlagsLinuxMetadata
	if share.ReadOnly {
		flags |= plan9ShareFlagsReadOnly
	}
	return flags
}
