package syslogd

import (
	"github.com/DSpeichert/netbootd/store"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/mcuadros/go-syslog.v2"
)

type Server struct {
	syslogServer *syslog.Server

	logger zerolog.Logger
	store  *store.Store
	addr   string
	ch     syslog.LogPartsChannel
}

func NewServer(store *store.Store, addr string) (server *Server, err error) {
	if addr == "" {
		addr = "0.0.0.0:514"
	}
	server = &Server{
		syslogServer: syslog.NewServer(),
		logger:       log.With().Str("service", "syslog").Logger(),
		store:        store,
		addr:         addr,
		ch:           make(syslog.LogPartsChannel),
	}

	server.syslogServer.SetFormat(syslog.Automatic)
	server.syslogServer.SetHandler(syslog.NewChannelHandler(server.ch))

	return server, nil
}

func (server *Server) Listen() error {
	err := server.syslogServer.ListenUDP(server.addr)
	if err != nil {
		return err
	}
	err = server.syslogServer.ListenTCP(server.addr)
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
