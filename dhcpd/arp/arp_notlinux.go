//go:build !linux
// +build !linux

package arp

import (
	"errors"
	"net"
)

// InjectArp injects an ARP entry into dev's ARP table
func InjectArp(ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	return errors.New("not implemented")
}

func InjectArpFd(fd uintptr, ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	return errors.New("not implemented")
}
