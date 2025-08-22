package syslogd

import (
	"github.com/DSpeichert/netbootd/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/mcuadros/go-syslog.v2"
)

var defaultAddr = "0.0.0.0:514"

type Server struct {
	syslogServer *syslog.Server

	logger zerolog.Logger
	store  *store.Store
	ch     syslog.LogPartsChannel
}

func NewServer(store *store.Store) (server *Server, err error) {
	server = &Server{
		syslogServer: syslog.NewServer(),
		logger:       log.With().Str("service", "syslog").Logger(),
		store:        store,
		ch:           make(syslog.LogPartsChannel),
	}

	server.syslogServer.SetFormat(syslog.Automatic)
	server.syslogServer.SetHandler(syslog.NewChannelHandler(server.ch))

	return server, nil
}

func (server *Server) ListenUDP(addr string) error {
	if addr == "" {
		addr = defaultAddr
	}
	err := server.syslogServer.ListenUDP(addr)
	if err != nil {
		return err
	}
	return nil
}

func (server *Server) ListenTCP(addr string) error {
	if addr == "" {
		addr = defaultAddr
	}
	err := server.syslogServer.ListenTCP(addr)
	if err != nil {
		return err
	}
	return nil
}

func (server *Server) Serve() error {
	err := server.syslogServer.Boot()
	if err != nil {
		return err
	}
	for logParts := range server.ch {
		server.syslogHandleLog(logParts)
	}
	return nil
}
