package httpd

import (
	"github.com/DSpeichert/netbootd/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net"
	"net/http"
	"time"
)

type Server struct {
	httpClient *http.Client
	httpServer *http.Server

	logger zerolog.Logger
	store  *store.Store
}

func NewServer(store *store.Store) (server *Server, err error) {

	server = &Server{
		httpServer: &http.Server{
			ReadTimeout:    10 * time.Second,
			MaxHeaderBytes: 1 << 20,
			IdleTimeout:    10 * time.Second,
		},
		logger: log.With().Str("service", "http").Logger(),
		store:  store,
	}

	server.httpServer.Handler = Handler{server: server}

	return server, nil
}

func (server *Server) Serve(l net.Listener) error {
	return server.httpServer.Serve(l)
}
