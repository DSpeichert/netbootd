package tftpd

import (
	"github.com/DSpeichert/netbootd/store"
	"github.com/pin/tftp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
	"net/http"
	"time"
)

type Server struct {
	httpClient *http.Client
	tftpServer *tftp.Server

	logger zerolog.Logger
	store  *store.Store
}

func NewServer(store *store.Store) (server *Server, err error) {

	server = &Server{
		httpClient: &http.Client{
			Timeout: time.Second * 30,
		},
		logger: log.With().Str("service", "tftp").Logger(),
		store:  store,
	}

	return server, nil
}

func (server *Server) Serve(conn *net.UDPConn) {
	server.tftpServer = tftp.NewServer(server.tftpReadHandler, nil)
	_ = server.tftpServer.Serve(conn)
}
