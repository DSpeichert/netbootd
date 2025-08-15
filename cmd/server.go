package cmd

import (
	"net"
	"os"
	"os/signal"

	"github.com/DSpeichert/netbootd/api"
	"github.com/DSpeichert/netbootd/config"
	"github.com/DSpeichert/netbootd/dhcpd"
	"github.com/DSpeichert/netbootd/httpd"
	"github.com/DSpeichert/netbootd/store"
	"github.com/DSpeichert/netbootd/tftpd"
	systemd "github.com/coreos/go-systemd/v22/daemon"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	addr         string
	ifname       string
	httpPort     int
	apiPort      int
	apiTlsCert   string
	apiTlsKey    string
	manifestPath string
	rootPath     string
)

func init() {
	serverCmd.Flags().StringVarP(&addr, "address", "a", "", "IP address to listen on (DHCP, TFTP, HTTP)")
	viper.BindPFlag("address", serverCmd.Flags().Lookup("address"))

	serverCmd.Flags().IntVarP(&httpPort, "http-port", "p", 8080, "HTTP port to listen on")
	viper.BindPFlag("http.port", serverCmd.Flags().Lookup("http-port"))

	serverCmd.Flags().IntVarP(&apiPort, "api-port", "r", 8081, "HTTP API port to listen on")
	viper.BindPFlag("api.port", serverCmd.Flags().Lookup("api-port"))

	serverCmd.Flags().StringVar(&apiTlsCert, "api-tls-cert", "", "Path to TLS certificate API")
	viper.BindPFlag("api.TLSCertificatePath", serverCmd.Flags().Lookup("api-tls-cert"))

	serverCmd.Flags().StringVar(&apiTlsKey, "api-tls-key", "", "Path to TLS certificate for API")
	viper.BindPFlag("api.TLSPrivateKeyPath", serverCmd.Flags().Lookup("api-tls-key"))

	serverCmd.Flags().StringVarP(&ifname, "interface", "i", "", "interface to listen on, e.g. eth0 (DHCP)")
	viper.BindPFlag("interface", serverCmd.Flags().Lookup("interface"))

	serverCmd.Flags().StringVarP(&manifestPath, "manifests", "m", "", "load manifests from directory")
	viper.BindPFlag("manifestPath", serverCmd.Flags().Lookup("manifests"))

	serverCmd.Flags().StringVarP(&rootPath, "root", "", "", "if not given as an absolute path, a mount's path.localDir is relative to this directory")
	viper.BindPFlag("rootPath", serverCmd.Flags().Lookup("root"))

	rootCmd.AddCommand(serverCmd)
}

var serverCmd = &cobra.Command{
	Use: "server",
	Run: func(cmd *cobra.Command, args []string) {
		// configure logging
		config.InitZeroLog()
		if viper.GetBool("trace") {
			zerolog.SetGlobalLevel(zerolog.TraceLevel)
		} else if viper.GetBool("debug") {
			zerolog.SetGlobalLevel(zerolog.DebugLevel)
		} else {
			zerolog.SetGlobalLevel(zerolog.InfoLevel)
		}

		// set up store
		store, _ := store.NewStore(store.Config{
			// TODO: config
			PersistenceDirectory: "",
		})
		if viper.GetString("manifestPath") != "" {
			log.Info().Str("path", viper.GetString("manifestPath")).Msg("Loading manifests")
			_ = store.LoadFromDirectory(viper.GetString("manifestPath"), viper.GetString("rootPath"))
		}
		store.GlobalHints.HttpPort = viper.GetInt("http.port")
		store.GlobalHints.ApiPort = viper.GetInt("api.port")

		// DHCP
		dhcpServer, err := dhcpd.NewServer(viper.GetString("address"), viper.GetString("interface"), store)
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create DHCP server")
		}
		go dhcpServer.Serve()

		// TFTP
		tftpServer, err := tftpd.NewServer(store, viper.GetString("rootPath"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create TFTP server")
		}
		connTftp, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.ParseIP(viper.GetString("address")),
			Port: 69, // TFTP
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to bind TFTP server")
		}
		go tftpServer.Serve(connTftp)

		// HTTP service
		httpServer, err := httpd.NewServer(store, viper.GetString("rootPath"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create HTTP server")
		}
		connHttp, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.ParseIP(viper.GetString("address")),
			Port: viper.GetInt("http.port"), // HTTP
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to bind HTTP API server")
		}
		go httpServer.Serve(connHttp)
		log.Info().Interface("addr", connHttp.Addr()).Msg("HTTP listening")

		// HTTP API service
		apiServer, err := api.NewServer(store, viper.GetString("api.authorization"), viper.GetString("rootPath"))
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to create HTTP API server")
		}
		connApi, err := net.ListenTCP("tcp", &net.TCPAddr{
			IP:   net.ParseIP(viper.GetString("address")),
			Port: viper.GetInt("api.port"), // HTTP
		})
		if err != nil {
			log.Fatal().Err(err).Msg("Failed to bind API server")
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
				err := apiServer.Serve(connApi)
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
