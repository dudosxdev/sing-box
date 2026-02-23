// Package mkcp implements the mKCP transport with finalmask UDP obfuscation.
// Ported from Xray-core/transport/internet/finalmask
package mkcp

import "net"

// ConnSize is implemented by mask connections that have a fixed overhead size.
type ConnSize interface {
	Size() int32
}

// Udpmask is a UDP packet mask that can wrap a PacketConn.
type Udpmask interface {
	WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error)
	WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error)
}

// UdpmaskManager manages a chain of UDP masks.
type UdpmaskManager struct {
	udpmasks []Udpmask
}

func NewUdpmaskManager(udpmasks []Udpmask) *UdpmaskManager {
	return &UdpmaskManager{udpmasks: udpmasks}
}

func (m *UdpmaskManager) WrapPacketConnClient(raw net.PacketConn) (net.PacketConn, error) {
	leaveSize := int32(0)
	var err error
	for i, mask := range m.udpmasks {
		raw, err = mask.WrapPacketConnClient(raw, i == len(m.udpmasks)-1, leaveSize, i == 0)
		if err != nil {
			return nil, err
		}
		leaveSize += raw.(ConnSize).Size()
	}
	return raw, nil
}

func (m *UdpmaskManager) WrapPacketConnServer(raw net.PacketConn) (net.PacketConn, error) {
	leaveSize := int32(0)
	var err error
	for i, mask := range m.udpmasks {
		raw, err = mask.WrapPacketConnServer(raw, i == len(m.udpmasks)-1, leaveSize, i == 0)
		if err != nil {
			return nil, err
		}
		leaveSize += raw.(ConnSize).Size()
	}
	return raw, nil
}
