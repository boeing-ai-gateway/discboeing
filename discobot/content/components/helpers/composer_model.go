package helpers

import (
	"net/url"
	"sort"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/discobot/internal/state"
	serverapi "github.com/obot-platform/discobot/server/api"
)

type ComposerModelProviderGroup struct {
	Provider string
	Models   []serverapi.ModelInfo
}

func ComposerModelsForWorkspace(data state.Data, selectedWorkspaceID string) []serverapi.ModelInfo {
	projectID := ComposerProjectIDForWorkspace(data, selectedWorkspaceID)
	var models []serverapi.ModelInfo
	for id, project := range data.Project {
		if projectID != "" && id != projectID {
			continue
		}
		models = append(models, project.Models...)
	}
	return ComposerSortedDedupedModels(models)
}

func ComposerProjectIDForWorkspace(data state.Data, selectedWorkspaceID string) string {
	for _, project := range data.Project {
		for _, workspace := range project.Workspaces {
			if workspace.ID == selectedWorkspaceID {
				return workspace.ProjectID
			}
		}
	}
	return ""
}

func ComposerSortedDedupedModels(models []serverapi.ModelInfo) []serverapi.ModelInfo {
	byProviderAndName := map[string]serverapi.ModelInfo{}
	for _, model := range models {
		cleanName := CleanModelName(model.Name)
		key := model.Provider + "::" + cleanName
		existing, ok := byProviderAndName[key]
		if !ok || strings.Contains(strings.ToLower(model.Name), "(latest)") {
			model.Name = cleanName
			byProviderAndName[key] = model
			continue
		}
		byProviderAndName[key] = existing
	}

	deduped := make([]serverapi.ModelInfo, 0, len(byProviderAndName))
	for _, model := range byProviderAndName {
		deduped = append(deduped, model)
	}
	sort.Slice(deduped, func(i, j int) bool {
		leftBase := ModelBaseName(deduped[i].Name)
		rightBase := ModelBaseName(deduped[j].Name)
		if leftBase != rightBase {
			return leftBase < rightBase
		}
		return deduped[i].Name > deduped[j].Name
	})
	return deduped
}

func ComposerModelProviderGroups(models []serverapi.ModelInfo) []ComposerModelProviderGroup {
	grouped := map[string][]serverapi.ModelInfo{}
	for _, model := range models {
		provider := model.Provider
		if provider == "" {
			provider = "Other"
		}
		grouped[provider] = append(grouped[provider], model)
	}

	providers := make([]string, 0, len(grouped))
	for provider := range grouped {
		providers = append(providers, provider)
	}
	sort.Strings(providers)

	groups := make([]ComposerModelProviderGroup, 0, len(providers))
	for _, provider := range providers {
		groups = append(groups, ComposerModelProviderGroup{Provider: provider, Models: grouped[provider]})
	}
	return groups
}

func ComposerSelectedModel(models []serverapi.ModelInfo, selectedModelID string) serverapi.ModelInfo {
	for _, model := range models {
		if model.ID == selectedModelID {
			return model
		}
	}
	return serverapi.ModelInfo{}
}

func ComposerSelectedModelForControls(models []serverapi.ModelInfo, selectedModelID string) serverapi.ModelInfo {
	selected := ComposerSelectedModel(models, selectedModelID)
	if selected.ID != "" {
		return selected
	}
	if len(models) > 0 {
		return models[0]
	}
	return serverapi.ModelInfo{}
}

func ComposerModelControlLabel(model serverapi.ModelInfo, composerState state.ComposerPanelState) string {
	if composerState.SelectedModelID == "" {
		return "Default model"
	}
	modelName := "Model"
	if model.Name != "" {
		modelName = model.Name
	}
	reasoning := ComposerReasoningButtonLabel(composerState.SelectedReasoning, ComposerDefaultReasoning(model))
	serviceTier := ComposerServiceTierLabel(composerState.SelectedServiceTier)
	return modelName + " · " + reasoning + " · " + serviceTier
}

func ComposerReasoningLevels(model serverapi.ModelInfo) []string {
	if model.ReasoningLevels == nil {
		return nil
	}
	return *model.ReasoningLevels
}

func ComposerServiceTiers(model serverapi.ModelInfo) []string {
	if model.ServiceTiers == nil {
		return nil
	}
	return *model.ServiceTiers
}

func ComposerDefaultReasoning(model serverapi.ModelInfo) string {
	if model.DefaultReasoning == nil {
		return ""
	}
	return *model.DefaultReasoning
}

func ComposerReasoningButtonLabel(reasoning string, defaultReasoning string) string {
	if reasoning == "" || reasoning == "default" {
		return ComposerReasoningLabel(defaultReasoning)
	}
	return ComposerReasoningLabel(reasoning)
}

func ComposerReasoningDefaultLabel(defaultReasoning string) string {
	if defaultReasoning == "" {
		return "Default"
	}
	return ComposerReasoningLabel(defaultReasoning) + " (default)"
}

func ComposerReasoningLabel(reasoning string) string {
	switch strings.ToLower(reasoning) {
	case "":
		return "Default"
	case "xhigh":
		return "X-High"
	default:
		return strings.ToUpper(reasoning[:1]) + reasoning[1:]
	}
}

func ComposerReasoningDescription(reasoning string) string {
	if reasoning == "none" {
		return "Use no reasoning effort"
	}
	return "Use " + reasoning + " reasoning effort"
}

func ComposerServiceTierLabel(serviceTier string) string {
	switch strings.ToLower(serviceTier) {
	case "priority", "fast":
		return "Fast"
	default:
		return "Standard"
	}
}

func ComposerServiceTierDescription(serviceTier string) string {
	switch strings.ToLower(serviceTier) {
	case "priority", "fast":
		return "Use the provider priority service tier"
	default:
		return "Use the " + serviceTier + " service tier"
	}
}

func ComposerModelSettingsSelectCommand(modelID string, reasoning string, serviceTier string) string {
	params := url.Values{}
	params.Set("model", modelID)
	params.Set("reasoning", reasoning)
	params.Set("serviceTier", serviceTier)
	return "@discobotCommand('/ui/commands/composer/model-settings/select?" + params.Encode() + "', {method: 'POST'})"
}

func ComposerReasoningPanelID(groupIndex int, modelIndex int) string {
	return "composer-model-menu-" + strconv.Itoa(groupIndex) + "-" + strconv.Itoa(modelIndex) + "-reasoning-panel"
}

func ComposerServiceTierPanelID(groupIndex int, modelIndex int, reasoning string) string {
	return "composer-model-menu-" + strconv.Itoa(groupIndex) + "-" + strconv.Itoa(modelIndex) + "-" + ComposerMenuIDPart(reasoning) + "-service-tier-panel"
}

func ComposerMenuIDPart(value string) string {
	if value == "" {
		return "default"
	}
	var builder strings.Builder
	for _, char := range strings.ToLower(value) {
		if (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9') {
			builder.WriteRune(char)
			continue
		}
		builder.WriteByte('-')
	}
	return strings.Trim(builder.String(), "-")
}

func CleanModelName(name string) string {
	return strings.TrimSpace(strings.ReplaceAll(name, "(latest)", ""))
}

func ModelBaseName(name string) string {
	name = strings.TrimSpace(name)
	fields := strings.Fields(name)
	for len(fields) > 0 && LooksVersionish(fields[len(fields)-1]) {
		fields = fields[:len(fields)-1]
	}
	return strings.Join(fields, " ")
}

func LooksVersionish(value string) bool {
	value = strings.TrimPrefix(strings.ToLower(value), "v")
	if value == "" {
		return false
	}
	for _, char := range value {
		if (char < '0' || char > '9') && char != '.' {
			return false
		}
	}
	return true
}
