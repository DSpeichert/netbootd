package tftpd

import (
	"net"
	"net/http"

	"github.com/DSpeichert/netbootd/store"
	"github.com/pin/tftp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Server struct {
	httpClient *http.Client
	tftpServer *tftp.Server

	logger   zerolog.Logger
	store    *store.Store
	rootPath string
}

func NewServer(store *store.Store, rootPath string) (server *Server, err error) {

	server = &Server{
		httpClient: &http.Client{},
		logger:     log.With().Str("service", "tftp").Logger(),
		store:      store,
		rootPath:   rootPath,
	}

	return server, nil
}

func (server *Server) Serve(conn *net.UDPConn) {
	server.tftpServer = tftp.NewServer(server.tftpReadHandler, nil)
	_ = server.tftpServer.Serve(conn)
}
