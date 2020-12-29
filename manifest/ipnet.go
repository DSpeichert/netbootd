package manifest

import (
	"net"
)

// An IPNet represents an IP network.
type IPWithNet struct {
	IP  net.IP
	Net net.IPNet
}

func (n *IPWithNet) String() string {
	return n.IP.String() + "/" + n.Net.Mask.String()
}

// MarshalText implements encoding.TextMarshaler using the
// standard CIDR representation of a IPNet.
func (n *IPWithNet) MarshalText() ([]byte, error) {
	return []byte(n.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (n *IPWithNet) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*n = IPWithNet{}
		return nil
	}

	ip, ipnet, err := net.ParseCIDR(string(text))
	if err != nil {
		return err
	}
	*n = IPWithNet{
		IP:  ip,
		Net: *ipnet,
	}
	return nil
}

func (n *IPWithNet) Netmask() string {
	return net.IP(n.Net.Mask).String()
}
