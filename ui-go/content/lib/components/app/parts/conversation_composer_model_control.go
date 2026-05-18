package parts

import (
	"slices"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func composerSelectedModelName(value string, models []viewmodel.ModelOption) string {
	for _, model := range models {
		if model.ID == value {
			return cleanComposerModelName(model.Name)
		}
	}
	return ""
}

func composerModelTitle(value string, models []viewmodel.ModelOption) string {
	if name := composerSelectedModelName(value, models); name != "" {
		return "Model: " + name
	}
	return "Model"
}

func composerModelProviders(models []viewmodel.ModelOption) []string {
	providers := make([]string, 0, len(models))
	for _, model := range models {
		provider := model.Provider
		if provider == "" {
			provider = "Other"
		}
		if !slices.Contains(providers, provider) {
			providers = append(providers, provider)
		}
	}
	slices.Sort(providers)
	return providers
}

func cleanComposerModelName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "(latest)", ""))
}

func composerModelCount(models []viewmodel.ModelOption) string {
	return strconv.Itoa(len(models))
}

func composerModelSelected(value string, modelID string) string {
	return strconv.FormatBool(value == modelID)
}
