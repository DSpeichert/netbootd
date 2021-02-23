package cmd

import (
	"fmt"
	"github.com/DSpeichert/netbootd/config"
	"github.com/spf13/cobra"
	"os"
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
	rootCmd.Flags().BoolVarP(&debug, "debug", "d", false, "enable debug logging")
	rootCmd.Flags().BoolVar(&trace, "trace", false, "enable trace logging")
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
		fmt.Println(err)
		os.Exit(1)
	}
}
