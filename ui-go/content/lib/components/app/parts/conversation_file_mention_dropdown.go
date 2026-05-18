package parts

import (
	"strings"

	"github.com/obot-platform/discobot/ui-go/content/lib/viewmodel"
)

func fileMentionShouldRender(snapshot viewmodel.FileMentionDropdownSnapshot) bool {
	return snapshot.Open && (snapshot.Loading || len(snapshot.Suggestions) > 0 || fileMentionShowEmpty(snapshot))
}

func fileMentionShowEmpty(snapshot viewmodel.FileMentionDropdownSnapshot) bool {
	return !snapshot.Loading && len(snapshot.Suggestions) == 0 && snapshot.Query != ""
}

func fileMentionRowClass(index int, selectedIndex int) string {
	base := "flex w-full items-center gap-2 px-3 py-1.5 text-left text-sm transition-colors"
	if index == selectedIndex {
		return base + " bg-accent"
	}
	return base + " hover:bg-accent"
}

func fileMentionDisplayPath(item viewmodel.FileMentionItem) string {
	if item.Type == "directory" {
		return item.Path + "/"
	}
	return item.Path
}

func fileMentionItemID(path string) string {
	replacer := strings.NewReplacer("/", "-", " ", "-", ".", "-", "_", "-")
	return "file-mention-" + replacer.Replace(path)
}

func fileMentionActiveID(snapshot viewmodel.FileMentionDropdownSnapshot) string {
	if snapshot.SelectedIndex < 0 || snapshot.SelectedIndex >= len(snapshot.Suggestions) {
		return ""
	}
	return fileMentionItemID(snapshot.Suggestions[snapshot.SelectedIndex].Path)
}
