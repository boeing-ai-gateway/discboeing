package classnames

import (
	"sync"

	twmerge "github.com/Oudwins/tailwind-merge-go/pkg/twmerge"
)

var (
	merge   = twmerge.CreateTwMerge(nil, nil)
	mergeMu sync.Mutex
)

// CN joins class strings and merges conflicting Tailwind utilities.
//
// Passing nil config and cache lets tailwind-merge-go initialize its default
// config and LRU cache on first use, matching the library README guidance.
func CN(parts ...string) string {
	mergeMu.Lock()
	defer mergeMu.Unlock()

	return merge(parts...)
}
