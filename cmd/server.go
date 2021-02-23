package cmd

import (
	"github.com/DSpeichert/netbootd/api"
	"github.com/DSpeichert/netbootd/dhcpd"
	"github.com/DSpeichert/netbootd/httpd"
	"github.com/DSpeichert/netbootd/store"
	"github.com/DSpeichert/netbootd/tftpd"
	systemd "github.com/coreos/go-systemd/daemon"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"net"
	"os"
	"os/signal"
)

var (
	addr         string
	ifname       string
	httpPort     int
	apiPort      int
	manifestPath string
)

func init() {
	serverCmd.Flags().StringVarP(&addr, "address", "a", "", "IP address to listen on (DHCP, TFTP, HTTP)")
	serverCmd.Flags().IntVarP(&httpPort, "http-port", "p", 8080, "HTTP port to listen on")
	serverCmd.Flags().IntVarP(&apiPort, "api-port", "r", 8081, "HTTP API port to listen on")
	serverCmd.Flags().StringVarP(&ifname, "interface", "i", "", "interface to listen on, e.g. eth0 (DHCP)")
	serverCmd.Flags().StringVarP(&manifestPath, "manifests", "m", "", "load manifests from directory")

	rootCmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		// configure logging
		if trace {
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		} else if debug {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}

		// set up store
		store, _ := store.NewStore(store.Config{
			// TODO: config
			PersistenceDirectory: "",
		})
		if manifestPath != "" {
			log.Info().Str("path", manifestPath).Msg("Loading manifests")
			_ = store.LoadFromDirectory(manifestPath)
		}
		store.GlobalHints.HttpPort = httpPort

		// DHCP
		dhcpServer, err := dhcpd.NewServer(addr, ifname, store)
		if err != nil {
			log.Fatal().Err(err)
		}
		go dhcpServer.Serve()

		// TFTP
		tftpServer, err := tftpd.NewServer(store)
		if err != nil {
			log.Fatal().Err(err)
		}
		connTftp, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(addr),
			Port: 69, // TFTP
		})
		if err != nil {
			log.Fatal().Err(err)
		}
		go tftpServer.Serve(connTftp)

		// HTTP service
		httpServer, err := httpd.NewServer(store)
		if err != nil {
			log.Fatal().Err(err)
		}
		connHttp, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.ParseIP(addr),
			Port: httpPort, // HTTP
		})
		if err != nil {
			log.Fatal().Err(err)
		}
		go httpServer.Serve(connHttp)
		log.Info().Interface("addr", connHttp.Addr()).Msg("HTTP listening")

		// HTTP API service
		apiServer, err := api.NewServer(store)
		if err != nil {
			log.Fatal().Err(err)
		}
		connApi, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.ParseIP(addr),
			Port: apiPort, // HTTP
		})
		if err != nil {
			log.Fatal().Err(err)
		}
		go apiServer.Serve(connApi)
		log.Info().Interface("api", connApi.Addr()).Msg("HTTP API listening")

		// notify systemd
		sent, err := systemd.SdNotify(true, "READY=1\n")
		if err != nil {
			log.Debug().Err(err).Msg("unable to send systemd daemon successful start message")
		} else if sent {
			log.Debug().Msg("systemd was notified.")
		} else {
			log.Debug().Msg("systemd notifications are not supported.")
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, os.Interrupt)
		<-sigs
	},
}
