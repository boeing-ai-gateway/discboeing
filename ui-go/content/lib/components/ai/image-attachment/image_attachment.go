package imageattachment

import "strings"

func imageFilename(filename string) string {
	if strings.TrimSpace(filename) == "" {
		return "Image attachment"
	}
	return filename
}
