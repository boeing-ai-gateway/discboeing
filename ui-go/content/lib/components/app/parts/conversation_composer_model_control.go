package parts

import (
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

var (
	composerLatestPattern        = regexp.MustCompile(`(?i)\s*\(latest\)\s*`)
	composerVersionPattern       = regexp.MustCompile(`(?i)\s+v\d+\s*`)
	composerTrailingNumberPatern = regexp.MustCompile(`\s+[\d.]+\s*$`)
	composerNumberPattern        = regexp.MustCompile(`\d+(?:\.\d+)?`)
)

func composerSelectedModelName(value string, models []viewmodel.ModelOption) string {
	for _, model := range composerDedupedModels(models) {
		if model.ID == value {
			return model.Name
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
	for _, model := range composerDedupedModels(models) {
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
	return strings.TrimSpace(composerLatestPattern.ReplaceAllString(name, ""))
}

func composerModelCount(models []viewmodel.ModelOption) string {
	return strconv.Itoa(len(models))
}

func composerModelSelected(value string, modelID string) string {
	return strconv.FormatBool(value == modelID)
}

func composerDedupedModels(models []viewmodel.ModelOption) []viewmodel.ModelOption {
	modelByProviderAndName := map[string]viewmodel.ModelOption{}
	for _, model := range models {
		cleanName := cleanComposerModelName(model.Name)
		provider := model.Provider
		if provider == "" {
			provider = "Other"
		}
		key := provider + "::" + cleanName
		_, exists := modelByProviderAndName[key]
		if !exists || strings.Contains(strings.ToLower(model.Name), "(latest)") {
			model.Name = cleanName
			modelByProviderAndName[key] = model
		}
	}

	deduped := make([]viewmodel.ModelOption, 0, len(modelByProviderAndName))
	for _, model := range modelByProviderAndName {
		deduped = append(deduped, model)
	}
	slices.SortFunc(deduped, func(left, right viewmodel.ModelOption) int {
		baseCompare := strings.Compare(composerModelBaseName(left.Name), composerModelBaseName(right.Name))
		if baseCompare != 0 {
			return baseCompare
		}
		leftVersion := composerModelVersion(left.Name)
		rightVersion := composerModelVersion(right.Name)
		if leftVersion != rightVersion {
			if rightVersion > leftVersion {
				return 1
			}
			return -1
		}
		return strings.Compare(left.Name, right.Name)
	})
	return deduped
}

func composerModelBaseName(name string) string {
	name = composerLatestPattern.ReplaceAllString(name, "")
	name = composerVersionPattern.ReplaceAllString(name, "")
	name = composerTrailingNumberPatern.ReplaceAllString(name, "")
	return strings.TrimSpace(name)
}

func composerModelVersion(name string) float64 {
	matches := composerNumberPattern.FindAllString(name, -1)
	if len(matches) == 0 {
		return 0
	}
	version, err := strconv.ParseFloat(matches[len(matches)-1], 64)
	if err != nil {
		return 0
	}
	return version
}

func composerModelOptionCommand(modelID string) string {
	return "@post('/ui/commands/composer-model?model=" + url.QueryEscape(modelID) + "')"
}
