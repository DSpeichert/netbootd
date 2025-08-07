// Package dhcpd contains snippets from MIT-licensed coredhcp project at
// https://github.com/coredhcp/coredhcp
package dhcpd

import (
	"net"

	"github.com/DSpeichert/netbootd/store"
	"github.com/insomniacslk/dhcp/dhcpv4/server4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/ipv4"
)

type Server struct {
	UdpConn *net.UDPConn
	*ipv4.PacketConn
	net.Interface
	address *net.UDPAddr
	logger  zerolog.Logger
	store   *store.Store
}

func NewServer(addr, ifname string, store *store.Store) (server *Server, err error) {
	server = &Server{
		address: &net.UDPAddr{
			IP:   net.ParseIP(addr),
			Port: 67,
			Zone: ifname,
		},
		logger: log.With().Str("service", "dhcpv4").Logger(),
		store:  store,
	}

	return server, nil
}

// MaxDatagram is the maximum length of message that can be received.
const MaxDatagram = 1 << 16

func (server *Server) Serve() {
	var err error

	// binds to specific interface (e.g. eth0) if provided
	server.UdpConn, err = server4.NewIPv4UDPConn(server.address.Zone, server.address)
	if err != nil {
		server.logger.Fatal().
			Err(err).
			Msgf("Cannot bind to %+v", server.address)
		return
	}
	server.PacketConn = ipv4.NewPacketConn(server.UdpConn)
	var ifi *net.Interface
	if server.address.Zone != "" {
		ifi, err = net.InterfaceByName(server.address.Zone)
		if err != nil {
			server.logger.Fatal().
				Err(err).
				Msg("could not find interface: " + server.address.Zone)
			return
		}
		server.Interface = *ifi
	} else {
		// When not bound to an interface, we need the information in each
		// packet to know which interface it came on
		err = server.SetControlMessage(ipv4.FlagInterface, true)
		if err != nil {
			server.logger.Fatal().
				Err(err).
				Msg("Cannot set control message when not specifying interface")
			return
		}
	}

	if server.address.IP.IsMulticast() {
		err = server.JoinGroup(ifi, server.address)
		if err != nil {
			server.logger.Fatal().
				Err(err).
				Msg("Cannot join multicast group")
			return
		}
	}

	log.Debug().Msgf("Listen %s", server.LocalAddr())
	for {
		b := make([]byte, MaxDatagram)

		n, oob, peer, err := server.ReadFrom(b)
		if err != nil {
			server.logger.
				Error().
				Err(err).
				Msg("error reading from connection")
		}
		go server.HandleMsg4(b[:n], oob, peer.(*net.UDPAddr))
	}
}
