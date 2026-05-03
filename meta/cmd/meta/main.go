package main

import (
	"log"

	"github.com/obot-platform/discobot/meta/internal/meta"
)

func main() {
	if err := meta.Run(); err != nil {
		log.Fatal(err)
	}
}
