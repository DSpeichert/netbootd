package arp

import (
	"syscall"
)

// ARP Flag values
// these are not in golang.org/x/sys/unix
const (
	// completed entry (ha valid)
	ATF_COM = 0x02
	// permanent entry
	ATF_PERM = 0x04
	// publish entry
	ATF_PUBL = 0x08
	// has requested trailers
	ATF_USETRAILERS = 0x10
	// want to use a netmask (only for proxy entries)
	ATF_NETMASK = 0x20
	// don't answer this addresses
	ATF_DONTPUB = 0x40
)

// https://man7.org/linux/man-pages/man7/arp.7.html
type arpReq struct {
	ArpPa   syscall.RawSockaddrInet4
	ArpHa   syscall.RawSockaddr
	Flags   int32
	Netmask syscall.RawSockaddr
	Dev     [16]byte
}
