//go:build linux && !amd64 && !arm64
// +build linux,!amd64,!arm64

package arp

import (
	"net"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

// InjectArp injects an ARP entry into dev's ARP table
// syscalls roughly based on https://www.unix.com/302447674-post3.html
// see:
//   https://github.com/torvalds/linux/blob/8cf8821e15cd553339a5b48ee555a0439c2b2742/net/ipv4/arp.c#L1179
//   https://github.com/torvalds/linux/blob/8cf8821e15cd553339a5b48ee555a0439c2b2742/net/ipv4/arp.c#L1024
func InjectArp(ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM, unix.IPPROTO_UDP)
	if err != nil {
		return
	}
	f := os.NewFile(uintptr(fd), "")
	defer f.Close()

	return InjectArpFd(uintptr(fd), ip, mac, flags, dev)
}

func InjectArpFd(fd uintptr, ip net.IP, mac net.HardwareAddr, flags int32, dev string) (err error) {
	arpReq := arpReq{
		ArpPa: syscall.RawSockaddrInet4{
			Family: syscall.AF_INET,
		},
		//Flags: 0x02 | 0x04, // ATF_COM | ATF_PERM;
		Flags: flags,
	}
	copy(arpReq.ArpPa.Addr[:], ip.To4())
	copy(arpReq.ArpHa.Data[:], mac)
	copy(arpReq.Dev[:], dev)

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, fd, unix.SIOCSARP, uintptr(unsafe.Pointer(&arpReq)))
	if errno != 0 {
		return errno
	}

	return
}
