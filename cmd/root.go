package cmd

import (
	"fmt"
	"os"

	"github.com/DSpeichert/netbootd/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	debug   bool
	trace   bool
	version string
	commit  string
	date    string
)

func init() {
	cobra.OnInitialize(config.InitConfig)
	rootCmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	viper.BindPFlag("debug", rootCmd.Flags().Lookup("debug"))

	rootCmd.PersistentFlags().BoolVar(&trace, "trace", false, "enable trace logging")
	viper.BindPFlag("trace", rootCmd.Flags().Lookup("trace"))

	rootCmd.PersistentFlags().BoolVar(&config.ZeroLogJournalDDisabled, "disable-journal-logger", false, "disable zerolog journald logger")
	viper.BindPFlag("disable-journal-logger", rootCmd.Flags().Lookup("disable-journal-logger"))

	rootCmd.PersistentFlags().BoolVar(&config.ZeroLogNoColor, "no-color", false, "disable colored logging")
	viper.BindPFlag("no-color", rootCmd.Flags().Lookup("no-color"))

	rootCmd.PersistentFlags().BoolVar(&config.ZeroLogEnableJSONLogger, "enable-json-logger", false, "enable JSON logging")
	viper.BindPFlag("enable-json-logger", rootCmd.Flags().Lookup("enable-json-logger"))
}

var rootCmd = &cobra.Command{
	Use:   "netbootd",
	Short: "netbootd is a DHCP/TFTP/HTTP minion",
	Long: `A programmable all-inclusive provisioning server including DHCP, TFTP and HTTP capability.
Unlike heavy, complex solutions like Foreman, netbootd is very lightweight and without many features,
allows for complete flexibility in provisioning machines.`,
	Version: version + " (" + commit + ") built " + date,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		config.InitZeroLog()
		fmt.Println(err)
		os.Exit(1)
	}
}
