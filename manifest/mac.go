// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.
//
// Standard net.HardwareAddr cannot be easily marshalled.
//
// See:
//   * https://github.com/golang/go/issues/29678
//   * https://go-review.googlesource.com/c/go/+/196817/

package manifest

import (
	"net"
)

// A HardwareAddr represents a physical hardware address.
type HardwareAddr []byte

func (a HardwareAddr) String() string {
	return net.HardwareAddr(a).String()
}

// ParseMAC parses s as an IEEE 802 MAC-48, EUI-48, EUI-64, or a 20-octet
// IP over InfiniBand link-layer address using one of the following formats:
//	00:00:5e:00:53:01
//	02:00:5e:10:00:00:00:01
//	00:00:00:00:fe:80:00:00:00:00:00:00:02:00:5e:10:00:00:00:01
//	00-00-5e-00-53-01
//	02-00-5e-10-00-00-00-01
//	00-00-00-00-fe-80-00-00-00-00-00-00-02-00-5e-10-00-00-00-01
//	0000.5e00.5301
//	0200.5e10.0000.0001
//	0000.0000.fe80.0000.0000.0000.0200.5e10.0000.0001
func ParseMAC(s string) (hw HardwareAddr, err error) {
	hwTmp, err := net.ParseMAC(s)
	if err != nil {
		return nil, err
	}
	return HardwareAddr(hwTmp), nil
}

// MarshalText implements encoding.TextMarshaler using the
// standard string representation of a HardwareAddr.
func (a HardwareAddr) MarshalText() ([]byte, error) {
	return []byte(a.String()), nil
}

// UnmarshalText implements encoding.TextUnmarshaler.
func (a *HardwareAddr) UnmarshalText(text []byte) error {
	if len(text) == 0 {
		*a = nil
		return nil
	}

	v, err := ParseMAC(string(text))
	if err != nil {
		return err
	}
	*a = v
	return nil
}
