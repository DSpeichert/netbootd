// +build !windows

package main

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
	"io"
	"os"
)

func init() {
	journalWriter := journald.NewJournalDWriter()
	multi := io.MultiWriter(zerolog.ConsoleWriter{Out: os.Stderr}, journalWriter)
	log.Logger = log.Output(multi)
	log.Debug().Msg("Enabled journald writer")
	journalLoggerEnabled = true
}
