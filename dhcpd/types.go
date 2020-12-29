package dhcpd

import (
	"fmt"
	"github.com/u-root/u-root/pkg/uio"
)

// Uint8 mirrors dhcpv4.Uint16
type Uint8 uint8

// ToBytes returns a serialized stream of bytes for this option.
func (o Uint8) ToBytes() []byte {
	buf := uio.NewBigEndianBuffer(nil)
	buf.Write8(uint8(o))
	return buf.Data()
}

// String returns a human-readable string for this option.
func (o Uint8) String() string {
	return fmt.Sprintf("%d", uint8(o))
}

// FromBytes decodes data into o as per RFC 2132, Section 9.10.
func (o *Uint8) FromBytes(data []byte) error {
	buf := uio.NewBigEndianBuffer(data)
	*o = Uint8(buf.Read8())
	return buf.FinError()
}
