package hvsocket

import (
	"encoding/binary"
	"fmt"

	"github.com/google/uuid"
)

var vsockTemplateTail = []byte{0xbd, 0x58, 0x64, 0x00, 0x6a, 0x79, 0x86, 0xd3}

func PortToServiceID(port int) (uuid.UUID, error) {
	if port < 1 || port > 0x7fffffff {
		return uuid.Nil, fmt.Errorf("VSOCK ports must fit in the first DWORD of the Hyper-V socket service GUID")
	}

	var id uuid.UUID
	binary.BigEndian.PutUint32(id[0:4], uint32(port))
	binary.BigEndian.PutUint16(id[4:6], 0xfacb)
	binary.BigEndian.PutUint16(id[6:8], 0x11e6)
	copy(id[8:], vsockTemplateTail)
	return id, nil
}

func MustPortToServiceID(port int) uuid.UUID {
	id, err := PortToServiceID(port)
	if err != nil {
		panic(err)
	}
	return id
}
