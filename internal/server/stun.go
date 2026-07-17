package server

import (
	"encoding/binary"
	"net"
)

const (
	stunBindingRequest = 0x0001
	stunBindingSuccess = 0x0101
	stunMagicCookie    = 0x2112a442
	stunXORMapped      = 0x0020
	stunHeaderLength   = 20
)

// stunResponse returns the minimal RFC 8489 Binding success response needed
// by WebRTC ICE gathering. Invalid or non-Binding datagrams are ignored.
func stunResponse(request []byte, remote *net.UDPAddr) ([]byte, bool) {
	if len(request) < stunHeaderLength || binary.BigEndian.Uint16(request[0:2]) != stunBindingRequest || binary.BigEndian.Uint32(request[4:8]) != stunMagicCookie {
		return nil, false
	}
	messageLength := int(binary.BigEndian.Uint16(request[2:4]))
	if messageLength%4 != 0 || stunHeaderLength+messageLength != len(request) {
		return nil, false
	}
	transactionID := request[8:20]
	ip := remote.IP
	family := byte(0x01)
	addressLength := 8
	if ip.To4() != nil {
		ip = ip.To4()
	} else if ip.To16() != nil {
		ip = ip.To16()
		family = 0x02
		addressLength = 20
	} else {
		return nil, false
	}
	response := make([]byte, stunHeaderLength+4+addressLength)
	binary.BigEndian.PutUint16(response[0:2], stunBindingSuccess)
	binary.BigEndian.PutUint16(response[2:4], uint16(4+addressLength))
	binary.BigEndian.PutUint32(response[4:8], stunMagicCookie)
	copy(response[8:20], transactionID)
	binary.BigEndian.PutUint16(response[20:22], stunXORMapped)
	binary.BigEndian.PutUint16(response[22:24], uint16(addressLength))
	response[25] = family
	binary.BigEndian.PutUint16(response[26:28], uint16(remote.Port)^uint16(stunMagicCookie>>16))
	if family == 0x01 {
		cookie := response[4:8]
		for i := 0; i < 4; i++ {
			response[28+i] = ip[i] ^ cookie[i]
		}
	} else {
		mask := response[4:20]
		for i := 0; i < 16; i++ {
			response[28+i] = ip[i] ^ mask[i]
		}
	}
	return response, true
}
