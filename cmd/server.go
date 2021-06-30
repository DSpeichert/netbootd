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
	"github.com/spf13/viper"
	"net"
	"os"
	"os/signal"
)

var (
	addr         string
	ifname       string
	httpPort     int
	apiPort      int
	apiTlsCert   string
	apiTlsKey    string
	manifestPath string
)

func init() {
	serverCmd.Flags().StringVarP(&addr, "address", "a", "", "IP address to listen on (DHCP, TFTP, HTTP)")
	serverCmd.Flags().IntVarP(&httpPort, "http-port", "p", 8080, "HTTP port to listen on")
	serverCmd.Flags().IntVarP(&apiPort, "api-port", "r", 8081, "HTTP API port to listen on")
	serverCmd.Flags().StringVar(&apiTlsCert, "api-tls-cert", "", "Path to TLS certificate API")
	serverCmd.Flags().StringVar(&apiTlsKey, "api-tls-key", "", "Path to TLS certificate for API")
	serverCmd.Flags().StringVarP(&ifname, "interface", "i", "", "interface to listen on, e.g. eth0 (DHCP)")
	serverCmd.Flags().StringVarP(&manifestPath, "manifests", "m", "", "load manifests from directory")

	viper.BindPFlag("api.TLSCertificatePath", serverCmd.Flags().Lookup("api-tls-cert"))
	viper.BindPFlag("api.TLSPrivateKeyPath", serverCmd.Flags().Lookup("api-tls-key"))
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
		apiServer, err := api.NewServer(store, viper.GetString("api.authorization"))
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
		if viper.GetString("api.TLSCertificatePath") != "" && viper.GetString("api.TLSPrivateKeyPath") != "" {
			log.Info().Interface("api", connApi.Addr()).Msg("HTTP API listening with TLS...")
			go func() {
				err := apiServer.ServeTLS(connApi, viper.GetString("api.TLSCertificatePath"), viper.GetString("api.TLSPrivateKeyPath"))
				log.Error().Err(err).Msg("Error initializing TLS HTTP API listener!")
			}()
		} else {
			go apiServer.Serve(connApi)
			log.Info().Interface("api", connApi.Addr()).Msg("HTTP API listening...")
			go func() {
				go apiServer.Serve(connApi)
				log.Error().Err(err).Msg("Error initializing HTTP API listener!")
			}()
		}
		if !viper.IsSet("api.authorization") {
			log.Warn().Interface("api", connApi.Addr()).Msg("API is running without authentication, set Authorization in config!")
		}

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
