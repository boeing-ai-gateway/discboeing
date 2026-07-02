package exedev

import (
	"fmt"
	"strings"

	"github.com/boeing-ai-gateway/discboeing/server/internal/sandbox"
)

const containerPort = 3002
const defaultEndpoint = "https://exe.dev/exec"
const defaultVMHostSuffix = "exe.xyz"
const defaultVMNamePrefix = "discboeing"
const defaultStopCommand = "ssh ${name} sudo shutdown -h now"
const defaultSandboxImage = "ghcr.io/boeing-ai-gateway/discboeing:main"
const stopCommandNamePlaceholder = "${name}"

type Config struct {
	Endpoint     string `json:"endpoint,omitempty"`
	Token        string `json:"token,omitempty"`
	VMHostSuffix string `json:"vmHostSuffix,omitempty"`
	VMNamePrefix string `json:"vmNamePrefix,omitempty"`
	StopCommand  string `json:"stopCommand,omitempty"`
	SandboxImage string `json:"sandboxImage,omitempty"`
}

func (c Config) withDefaults() Config {
	c.Token = strings.TrimSpace(c.Token)
	if c.Endpoint == "" {
		c.Endpoint = defaultEndpoint
	}
	if c.VMHostSuffix == "" {
		c.VMHostSuffix = defaultVMHostSuffix
	}
	if c.VMNamePrefix == "" {
		c.VMNamePrefix = defaultVMNamePrefix
	}
	if c.StopCommand == "" {
		c.StopCommand = defaultStopCommand
	}
	if c.SandboxImage == "" {
		c.SandboxImage = defaultSandboxImage
	}
	return c
}

func Definition() sandbox.ProviderDefinition {
	return sandbox.ProviderDefinition{
		Name:        "exe.dev",
		Icon:        "https://exe.dev/static/exy.png",
		Description: "exe.dev VM sandbox driver",
		ConfigFields: []sandbox.ProviderConfigField{
			{Key: "endpoint", Label: "Command endpoint", Type: "text", Placeholder: defaultEndpoint, Description: "HTTPS endpoint used by Discboeing to issue exe.dev commands.", Advanced: true},
			{Key: "credentialId", Label: "API credential", Type: "credential", Description: "Credential containing the exe.dev API token.", Required: true, CredentialProvider: "exedev", CredentialAuthType: "api_key"},
			{Key: "vmHostSuffix", Label: "VM host suffix", Type: "text", Placeholder: defaultVMHostSuffix, Description: "DNS suffix used to reach created VMs.", Advanced: true},
			{Key: "vmNamePrefix", Label: "VM name prefix", Type: "text", Placeholder: defaultVMNamePrefix, Description: "Prefix for VMs created by Discboeing.", Advanced: true},
			{Key: "stopCommand", Label: "Stop command", Type: "textarea", Placeholder: defaultStopCommand, Description: "Optional command template used when stopping a VM. ${name} is replaced with the quoted VM name.", Advanced: true},
			{Key: "sandboxImage", Label: "Sandbox image", Type: "text", Placeholder: defaultSandboxImage, Description: "Optional sandbox image override for this provider instance. Leave blank to use the remote sandbox image setting.", Advanced: true},
		},
	}
}

func requireConfig(cfg Config) error {
	if cfg.Endpoint == "" {
		return fmt.Errorf("exe.dev endpoint is required")
	}
	if cfg.Token == "" {
		return fmt.Errorf("exe.dev token is required")
	}
	return nil
}
