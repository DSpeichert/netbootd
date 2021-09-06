// Package dhcpd contains snippets from MIT-licensed coredhcp project at
// https://github.com/coredhcp/coredhcp
package dhcpd

import (
	"errors"
	"net"
	"strings"

	mfest "github.com/DSpeichert/netbootd/manifest"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"golang.org/x/net/ipv4"
)

func (server *Server) HandleMsg4(buf []byte, oob *ipv4.ControlMessage, peer net.Addr) {
	var (
		resp         *dhcpv4.DHCPv4
		err          error
		bootFileSize int
		manifest     *mfest.Manifest
	)

	req, err := dhcpv4.FromBytes(buf)
	if err != nil {
		server.logger.Error().Err(err).Msg("Error parsing DHCPv4 request")
		return
	}

	server.logger.Trace().
		Str("peer", peer.String()).
		Interface("request", req.Summary()).
		Msg("Received DHCP packet")

	if req.OpCode != dhcpv4.OpcodeBootRequest {
		server.logger.Error().
			Int("opcode", int(req.OpCode)).
			Msg("unsupported opcode")
		return
	}

	resp, err = dhcpv4.NewReplyFromRequest(req)
	if err != nil {
		server.logger.Error().
			Err(err).
			Msg("failed to build reply")
		return
	}

	switch mt := req.MessageType(); mt {
	case dhcpv4.MessageTypeDiscover:
		resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeOffer))
	case dhcpv4.MessageTypeRequest:
		resp.UpdateOption(dhcpv4.OptMessageType(dhcpv4.MessageTypeAck))
	case dhcpv4.MessageTypeRelease:
		return
	default:
		server.logger.Error().
			Str("type", mt.String()).
			Msg("unknown message type")
		return
	}

	// find local IP
	ifIndex := server.Interface.Index
	if ifIndex == 0 && oob != nil {
		ifIndex = oob.IfIndex
	}
	localIp, err := getIpv4ForInterface(ifIndex)
	if err != nil {
		server.logger.Error().
			Err(err).
			Msg("failed to find local interface")
		resp = nil
		goto response
	}

	manifest = server.store.FindByMAC(req.ClientHWAddr)
	if manifest == nil {
		server.logger.Info().
			Str("MAC", req.ClientHWAddr.String()).
			Msg("ignore packet from unknown MAC")
		resp = nil
		goto response
	}

	// server ID
	if req.ServerIPAddr != nil &&
		!req.ServerIPAddr.Equal(net.IPv4zero) &&
		!req.ServerIPAddr.Equal(localIp) {
		server.logger.Trace().
			Msg("requested server ID does not match this server's ID")
		resp = nil
		goto response
	} else {
		resp.ServerIPAddr = make(net.IP, net.IPv4len)
		copy(resp.ServerIPAddr[:], localIp)
		resp.UpdateOption(dhcpv4.OptServerIdentifier(localIp))
	}

	resp.YourIPAddr = manifest.IPv4.IP
	resp.Options.Update(dhcpv4.OptSubnetMask(manifest.IPv4.Net.Mask))

	// lease time
	if req.OpCode == dhcpv4.OpcodeBootRequest && manifest.LeaseDuration != 0 {
		resp.Options.Update(dhcpv4.OptIPAddressLeaseTime(manifest.LeaseDuration))
	}

	// hostname
	if req.IsOptionRequested(dhcpv4.OptionHostName) {
		resp.Options.Update(dhcpv4.OptHostName(manifest.Hostname))
	}

	// dns
	if req.IsOptionRequested(dhcpv4.OptionDomainNameServer) {
		resp.Options.Update(dhcpv4.OptDNS(manifest.DNS...))
	}

	// router
	if req.IsOptionRequested(dhcpv4.OptionRouter) {
		resp.Options.Update(dhcpv4.OptRouter(manifest.Router...))
	}

	// NTP
	if req.IsOptionRequested(dhcpv4.OptionNTPServers) {
		resp.Options.Update(dhcpv4.OptNTPServers(manifest.NTP...))
	}

	// NBP
	if req.IsOptionRequested(dhcpv4.OptionTFTPServerName) && !manifest.Suspended {
		resp.Options.Update(dhcpv4.OptTFTPServerName(localIp.String()))
	}

	if req.IsOptionRequested(dhcpv4.OptionBootfileName) && !manifest.Suspended {
		// serve iPXE script if user-class is iPXE, or whatever the user chooses if iPXE is disabled
		if stringSlicesEqual(req.UserClass(), []string{"iPXE"}) || !manifest.Ipxe {
			resp.Options.Update(dhcpv4.OptBootFileName(manifest.BootFilename))
		} else if len(req.ClientArch()) > 0 && req.ClientArch()[0] > 0 {
			// likely UEFI (not BIOS)
			if strings.Contains(req.ClassIdentifier(), "PXEClient:Arch:00011") {
				resp.Options.Update(dhcpv4.OptBootFileName("ipxe_arm64.efi"))
			} else {
				resp.Options.Update(dhcpv4.OptBootFileName("ipxe.efi"))
			}
			//bootFileSize = 1
		} else {
			resp.Options.Update(dhcpv4.OptBootFileName("undionly.kpxe"))
			//bootFileSize = 1
		}
	}

	if req.IsOptionRequested(dhcpv4.OptionBootFileSize) && bootFileSize > 0 {
		resp.Options.Update(dhcpv4.Option{
			Code:  dhcpv4.OptionBootFileSize,
			Value: Uint8(bootFileSize),
		})
	}

	// iPXE specific
	if stringSlicesEqual(req.UserClass(), []string{"iPXE"}) {
		resp.Options.Update(dhcpv4.Option{
			Code:  dhcpv4.GenericOptionCode(176), // ipxe.no-pxedhcp
			Value: dhcpv4.Uint16(1),              // should be uint8 according to iPXE docs
		})
	}

response:
	// continue main handler
	if resp != nil {
		var peer *net.UDPAddr
		if !req.GatewayIPAddr.IsUnspecified() {
			// TODO: make RFC8357 compliant
			peer = &net.UDPAddr{IP: req.GatewayIPAddr, Port: dhcpv4.ServerPort}
		} else if resp.MessageType() == dhcpv4.MessageTypeNak {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else if !req.ClientIPAddr.IsUnspecified() {
			peer = &net.UDPAddr{IP: req.ClientIPAddr, Port: dhcpv4.ClientPort}
		} else if req.IsBroadcast() {
			peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
		} else {
			// we must inject ARP to unicast to IP/MAC that's not on the network yet
			device := server.Interface.Name
			if device == "" && oob != nil && oob.IfIndex != 0 {
				if netif, err := net.InterfaceByIndex(oob.IfIndex); err == nil {
					device = netif.Name
				}
			}
			rawConn, err := server.UdpConn.SyscallConn()
			if device != "" && err == nil {
				rawConn.Control(func(fd uintptr) {
					err = InjectArpFd(fd, resp.YourIPAddr, req.ClientHWAddr, ATF_COM, device)
				})
				if err != nil {
					server.logger.Error().
						Err(err).
						Msg("ioctl failed")
				}
			}

			if device != "" && err == nil {
				peer = &net.UDPAddr{IP: resp.YourIPAddr, Port: dhcpv4.ClientPort}
			} else {
				// fall back to broadcast
				peer = &net.UDPAddr{IP: net.IPv4bcast, Port: dhcpv4.ClientPort}
			}
		}

		var woob *ipv4.ControlMessage
		if peer.IP.Equal(net.IPv4bcast) || peer.IP.IsLinkLocalUnicast() {
			// Direct broadcasts and link-local to the interface the request was
			// received on. Other packets should use the normal routing table in
			// case of asymmetric routing.
			switch {
			case server.Interface.Index != 0:
				woob = &ipv4.ControlMessage{IfIndex: server.Interface.Index}
			case oob != nil && oob.IfIndex != 0:
				woob = &ipv4.ControlMessage{IfIndex: oob.IfIndex}
			default:
				server.logger.Error().
					Str("peer", peer.String()).
					Msg("did not receive interface information")
			}
		}

		server.logger.Debug().
			Interface("response", resp).
			Msg("sending DHCP packet")

		if _, err := server.WriteTo(resp.ToBytes(), woob, peer); err != nil {
			server.logger.Error().
				Err(err).
				Str("peer", peer.String()).
				Msg("conn.Write failed")
		}

	} else {
		server.logger.Trace().
			Msg("dropping request because response is nil")
	}
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func getIpv4ForInterface(i int) (net.IP, error) {
	netif, err := net.InterfaceByIndex(i)
	if err != nil {
		return nil, err
	}
	addresses, err := netif.Addrs()
	if err != nil {
		return nil, err
	}
	for _, address := range addresses {
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.To4(), nil
			}
		}
	}
	return nil, errors.New("no IP found")
}
