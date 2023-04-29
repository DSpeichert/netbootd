package config

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
)

var (
	zerologInitDone        bool // to prevent zerolog to be initialized twice in specific situations (like parsing error of viper configuration file)
	ZeroLogJournalDEnabled bool // used by viper to store the status of the --disable-journal-logger flag
)

func init() {
	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func InitZeroLog() {
	if !zerologInitDone {
		if ZeroLogJournalDEnabled {
			log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
			log.Debug().Msg("Enabled consoler writer")
		} else {
			journalWriter := journald.NewJournalDWriter()
			multi := io.MultiWriter(zerolog.ConsoleWriter{Out: os.Stderr}, journalWriter)
			log.Logger = log.Output(multi)
			log.Debug().Msg("Enabled journald writer")
		}
		zerologInitDone = true
	}
}
