package linksafetymodal

import "strings"

func safeURLLabel(url string) string {
	if strings.TrimSpace(url) == "" {
		return "external link"
	}
	return url
}
