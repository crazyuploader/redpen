// cmd/redpen/main.go
package main

import (
	"os"

	"github.com/rs/zerolog/log"
)

func main() {
	if err := Execute(); err != nil {
		log.Error().Err(err).Msg("command failed")
		os.Exit(1)
	}
}
