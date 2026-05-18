package proxy

import (
	"encoding/binary"
	"net"
)

var proxyProtocolV2Signature = [12]byte{
	0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A,
}

const (
	proxyProtocolV2VersionCommand = 0x21 // version 2, PROXY command

	proxyProtocolV2FamilyTCP4 = 0x11 // AF_INET + STREAM
	proxyProtocolV2FamilyTCP6 = 0x21 // AF_INET6 + STREAM
	proxyProtocolV2FamilyUDP4 = 0x12 // AF_INET + DGRAM
	proxyProtocolV2FamilyUDP6 = 0x22 // AF_INET6 + DGRAM

	proxyProtocolV2IPv4AddrLen = 12 // 4+4+2+2
	proxyProtocolV2IPv6AddrLen = 36 // 16+16+2+2
)

func buildProxyProtocolV2Header(srcIP net.IP, dstIP net.IP, srcPort int, dstPort int, stream bool) []byte {
	srcIsIPv4 := srcIP.To4() != nil
	dstIsIPv4 := dstIP.To4() != nil
	if srcIsIPv4 != dstIsIPv4 {
		return nil
	}
	isIPv4 := srcIsIPv4

	var family byte
	var addrLen int
	if isIPv4 {
		addrLen = proxyProtocolV2IPv4AddrLen
		if stream {
			family = proxyProtocolV2FamilyTCP4
		} else {
			family = proxyProtocolV2FamilyUDP4
		}
	} else {
		addrLen = proxyProtocolV2IPv6AddrLen
		if stream {
			family = proxyProtocolV2FamilyTCP6
		} else {
			family = proxyProtocolV2FamilyUDP6
		}
	}

	header := make([]byte, 16+addrLen)
	copy(header[0:12], proxyProtocolV2Signature[:])
	header[12] = proxyProtocolV2VersionCommand
	header[13] = family
	binary.BigEndian.PutUint16(header[14:16], uint16(addrLen))

	offset := 16
	if isIPv4 {
		copy(header[offset:offset+4], srcIP.To4())
		copy(header[offset+4:offset+8], dstIP.To4())
		binary.BigEndian.PutUint16(header[offset+8:offset+10], uint16(srcPort))
		binary.BigEndian.PutUint16(header[offset+10:offset+12], uint16(dstPort))
	} else {
		copy(header[offset:offset+16], srcIP.To16())
		copy(header[offset+16:offset+32], dstIP.To16())
		binary.BigEndian.PutUint16(header[offset+32:offset+34], uint16(srcPort))
		binary.BigEndian.PutUint16(header[offset+34:offset+36], uint16(dstPort))
	}

	return header
}
