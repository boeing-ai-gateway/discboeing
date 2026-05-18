package parts

import (
	"slices"
	"strconv"
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func slashCommandSuggestions(snapshot viewmodel.SlashCommandDropdownSnapshot) []viewmodel.SlashCommand {
	query := strings.TrimSpace(strings.ToLower(snapshot.Query))
	commands := slices.Clone(snapshot.Commands)
	slices.SortFunc(commands, func(left, right viewmodel.SlashCommand) int {
		if left.Order != right.Order {
			return left.Order - right.Order
		}
		return strings.Compare(left.Name, right.Name)
	})
	if query == "" {
		return commands
	}

	suggestions := commands[:0]
	for _, command := range commands {
		if strings.Contains(strings.ToLower(command.Name), query) || strings.Contains(strings.ToLower(command.Description), query) {
			suggestions = append(suggestions, command)
		}
	}
	return suggestions
}

func slashCommandShouldRender(snapshot viewmodel.SlashCommandDropdownSnapshot) bool {
	if !snapshot.Open || snapshot.SessionID == "" {
		return false
	}

	return snapshot.Loading || len(slashCommandSuggestions(snapshot)) > 0 || slashCommandShowEmpty(snapshot)
}

func slashCommandShowEmpty(snapshot viewmodel.SlashCommandDropdownSnapshot) bool {
	return snapshot.Open && snapshot.SessionID != "" && !snapshot.Loading && len(slashCommandSuggestions(snapshot)) == 0
}

func slashCommandEmptyMessage(query string) string {
	if strings.TrimSpace(query) == "" {
		return "No commands available"
	}

	return "No commands match “" + query + "”"
}

func slashCommandItemClass(index int, selectedIndex int) string {
	className := "flex w-full items-start gap-2 px-3 py-2 text-left text-sm transition-colors "
	if index == selectedIndex {
		return className + "bg-accent"
	}

	return className + "hover:bg-accent"
}

func slashCommandIndex(index int) string {
	return strconv.Itoa(index)
}

func slashCommandItemID(name string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", "_", "-", ".", "-")
	return "slash-command-" + replacer.Replace(name)
}

func slashCommandActiveID(snapshot viewmodel.SlashCommandDropdownSnapshot) string {
	suggestions := slashCommandSuggestions(snapshot)
	if snapshot.SelectedIndex < 0 || snapshot.SelectedIndex >= len(suggestions) {
		return ""
	}
	return slashCommandItemID(suggestions[snapshot.SelectedIndex].Name)
}
