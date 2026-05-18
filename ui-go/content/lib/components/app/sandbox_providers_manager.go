package app

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func sandboxProviderName(provider viewmodel.SandboxProviderInstance) string {
	if strings.TrimSpace(provider.Name) != "" {
		return provider.Name
	}
	if strings.TrimSpace(provider.Type) != "" {
		return provider.Type
	}
	return "Provider"
}

func sandboxProviderTypeName(providerType viewmodel.SandboxProviderType) string {
	if strings.TrimSpace(providerType.Name) != "" {
		return providerType.Name
	}
	return providerType.ID
}

func sandboxProviderMonogram(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "P"
	}
	return strings.ToUpper(name[:1])
}

func sandboxProviderDefaultLabel(snapshot viewmodel.SandboxProvidersManagerSnapshot) string {
	if snapshot.ProjectDefaultID != "" {
		return "Project default: " + sandboxProviderDisplayName(snapshot, snapshot.ProjectDefaultID)
	}
	if snapshot.DefaultProviderID != "" {
		return "Effective default: " + sandboxProviderDisplayName(snapshot, snapshot.DefaultProviderID)
	}
	return "No default provider is available"
}

func sandboxProviderDisplayName(snapshot viewmodel.SandboxProvidersManagerSnapshot, id string) string {
	for _, provider := range snapshot.Providers {
		if provider.ID == id {
			return sandboxProviderName(provider)
		}
	}
	return id
}

func sandboxProviderDescription(snapshot viewmodel.SandboxProvidersManagerSnapshot, provider viewmodel.SandboxProviderInstance) string {
	parts := []string{"Driver: " + provider.Type}
	if provider.ID == snapshot.DefaultProviderID {
		parts = append(parts, "default")
	}
	if provider.BuiltIn {
		parts = append(parts, "built-in")
	}
	if provider.Disabled {
		parts = append(parts, "disabled")
	} else if !provider.Available {
		parts = append(parts, "unavailable")
	}
	if provider.Description != "" {
		parts = append(parts, provider.Description)
	}
	return strings.Join(parts, " · ")
}

func sandboxProviderTypeDescription(providerType viewmodel.SandboxProviderType) string {
	parts := []string{"Driver: " + providerType.ID}
	if providerType.Description != "" {
		parts = append(parts, providerType.Description)
	}
	return strings.Join(parts, " · ")
}

func sandboxProviderCanOpenControls(provider viewmodel.SandboxProviderInstance) bool {
	return (provider.Capabilities.Resources || provider.Capabilities.Inspection) && provider.Available && !provider.Disabled
}

func sandboxAvailableProviderTypes(snapshot viewmodel.SandboxProvidersManagerSnapshot) []viewmodel.SandboxProviderType {
	available := []viewmodel.SandboxProviderType{}
	for _, providerType := range snapshot.ProviderTypes {
		if providerType.Available {
			available = append(available, providerType)
		}
	}
	return available
}

func sandboxProviderManagerView(snapshot viewmodel.SandboxProvidersManagerSnapshot) string {
	switch {
	case snapshot.DriverPickerOpen:
		return "driver-picker"
	case snapshot.FormOpen:
		return "form"
	case snapshot.RuntimeProviderID != "":
		return "runtime-controls"
	default:
		return "list"
	}
}

func sandboxProviderRuntimeIcon(snapshot viewmodel.SandboxProvidersManagerSnapshot) string {
	for _, provider := range snapshot.Providers {
		if provider.ID == snapshot.RuntimeProviderID {
			return provider.Icon
		}
	}
	return ""
}

func sandboxProviderRefreshIconClass(loading bool) string {
	if loading {
		return "size-4 animate-spin"
	}
	return "size-4"
}

func sandboxProviderToggleLabel(provider viewmodel.SandboxProviderInstance) string {
	if provider.Disabled {
		return "Enable " + sandboxProviderName(provider)
	}
	return "Disable " + sandboxProviderName(provider)
}

func sandboxProviderSwitchClass(provider viewmodel.SandboxProviderInstance) string {
	base := "relative inline-flex h-5 w-9 shrink-0 items-center rounded-full border transition-colors disabled:cursor-default "
	if provider.Disabled {
		return base + "border-border bg-muted"
	}
	return base + "border-primary bg-primary"
}

func sandboxProviderSwitchThumbClass(provider viewmodel.SandboxProviderInstance) string {
	base := "block size-4 rounded-full bg-background shadow transition-transform "
	if provider.Disabled {
		return base + "translate-x-0.5"
	}
	return base + "translate-x-4"
}

func sandboxProviderConfigFieldID(field viewmodel.SandboxProviderConfigField) string {
	return "sandbox-provider-config-" + field.Key
}

func sandboxProviderInputType(field viewmodel.SandboxProviderConfigField) string {
	if field.Type == "number" {
		return "number"
	}
	return "text"
}
