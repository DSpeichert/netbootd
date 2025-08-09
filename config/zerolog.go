package config

import (
	"io"
	"os"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/journald"
	"github.com/rs/zerolog/log"
)

var (
	zerologInitDone         bool // to prevent zerolog to be initialized twice in specific situations (like parsing error of viper configuration file)
	ZeroLogJournalDDisabled bool // used by viper to store the status of the --disable-journal-logger flag
	ZeroLogNoColor          bool // used by viper to store the status of the --no-color flag
	ZeroLogEnableJSONLogger bool // used by viper to store the status of the --enable-json-logger flag
)

func init() {
	// UNIX Time is faster and smaller than most timestamps
	// If you set zerolog.TimeFieldFormat to an empty string,
	// logs will write with UNIX time
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
}

func InitZeroLog() {
	if !zerologInitDone {
		var output io.Writer
		output = os.Stderr
		if !ZeroLogEnableJSONLogger {
			output = zerolog.ConsoleWriter{Out: os.Stderr, NoColor: ZeroLogNoColor}
		}
		if ZeroLogJournalDDisabled {
			log.Logger = log.Output(output)
			log.Debug().Msg("Enabled console writer")
		} else {
			journalWriter := journald.NewJournalDWriter()
			multi := io.MultiWriter(output, journalWriter)
			log.Logger = log.Output(multi)
			log.Debug().Msg("Enabled journald writer")
		}
		zerologInitDone = true
	}
}
