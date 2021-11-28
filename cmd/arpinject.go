package cmd

import (
	"github.com/DSpeichert/netbootd/dhcpd/arp"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"net"
)

var (
	mac    string
	ip     string
	device string
)

func init() {
	arpInjectCmd.Flags().StringVarP(&ip, "ip", "i", "", "IP Address")
	arpInjectCmd.Flags().StringVarP(&mac, "mac", "m", "", "MAC address")
	arpInjectCmd.Flags().StringVarP(&device, "device", "d", "", "device")

	//rootCmd.AddCommand(arpInjectCmd)
}

var arpInjectCmd = &cobra.Command{
	Use: "arpinject",
	Run: func(cmd *cobra.Command, args []string) {
		parsedIp := net.ParseIP(ip)
		parsedMac, err := net.ParseMAC(mac)
		if err != nil {
			log.Error().
				Err(err).
				Msg("cannot parse mac")
		}
		if err = arp.InjectArp(parsedIp, parsedMac, arp.ATF_COM, device); err != nil {
			log.Error().
				Err(err).
				Msg("cannot inject arp entry")
		}
	},
}
