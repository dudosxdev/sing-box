package mkcp

import (
	"encoding/binary"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"
)

// headerConn is a generic header-based mask connection.
type headerConn struct {
	first     bool
	leaveSize int32
	conn      net.PacketConn
	headerBuf []byte
	headerLen int32

	readBuf    []byte
	readMutex  sync.Mutex
	writeBuf   []byte
	writeMutex sync.Mutex

	serializeHeader func(b []byte)
}

func newHeaderConn(raw net.PacketConn, first bool, leaveSize int32, headerLen int32, serializeHeader func(b []byte)) *headerConn {
	c := &headerConn{
		first:           first,
		leaveSize:       leaveSize,
		conn:            raw,
		headerLen:       headerLen,
		serializeHeader: serializeHeader,
	}
	if first {
		c.readBuf = make([]byte, 8192)
		c.writeBuf = make([]byte, 8192)
	}
	return c
}

func (c *headerConn) Size() int32 { return c.headerLen }

func (c *headerConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	if c.first {
		c.readMutex.Lock()
		n, addr, err = c.conn.ReadFrom(c.readBuf)
		if err != nil {
			c.readMutex.Unlock()
			return n, addr, err
		}
		if n < int(c.Size()) {
			c.readMutex.Unlock()
			return 0, addr, io.ErrShortBuffer
		}
		if len(p) < n-int(c.Size()) {
			c.readMutex.Unlock()
			return 0, addr, io.ErrShortBuffer
		}
		copy(p, c.readBuf[c.Size():n])
		c.readMutex.Unlock()
		return n - int(c.Size()), addr, err
	}

	n, addr, err = c.conn.ReadFrom(p)
	if err != nil {
		return n, addr, err
	}
	if n < int(c.Size()) {
		return 0, addr, io.ErrShortBuffer
	}
	copy(p, p[c.Size():n])
	return n - int(c.Size()), addr, err
}

func (c *headerConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.first {
		if c.leaveSize+c.Size()+int32(len(p)) > 8192 {
			return 0, &maskError{"too many masks"}
		}
		c.writeMutex.Lock()
		n = copy(c.writeBuf[c.leaveSize+c.Size():], p)
		n += int(c.leaveSize) + int(c.Size())
		c.serializeHeader(c.writeBuf[c.leaveSize : c.leaveSize+c.Size()])
		nn, err := c.conn.WriteTo(c.writeBuf[:n], addr)
		if err != nil {
			c.writeMutex.Unlock()
			return 0, err
		}
		if nn != n {
			c.writeMutex.Unlock()
			return 0, &maskError{"nn != n"}
		}
		c.writeMutex.Unlock()
		return len(p), nil
	}
	c.serializeHeader(p[c.leaveSize : c.leaveSize+c.Size()])
	return c.conn.WriteTo(p, addr)
}

func (c *headerConn) Close() error                       { return c.conn.Close() }
func (c *headerConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *headerConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *headerConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *headerConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }

// --- SRTP ---

type srtpMask struct{}

func newSRTPMask() Udpmask { return &srtpMask{} }

func (*srtpMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	header := uint16(0xB5E8)
	number := uint16(rand.Uint32())
	return newHeaderConn(raw, first, leaveSize, 4, func(b []byte) {
		number++
		binary.BigEndian.PutUint16(b, header)
		binary.BigEndian.PutUint16(b[2:], number)
	}), nil
}

func (*srtpMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return (*srtpMask)(nil).WrapPacketConnClient(raw, first, leaveSize, end)
}

// --- UTP ---

type utpMask struct{}

func newUTPMask() Udpmask { return &utpMask{} }

func (*utpMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	connectionID := uint16(rand.Uint32())
	return newHeaderConn(raw, first, leaveSize, 4, func(b []byte) {
		binary.BigEndian.PutUint16(b, connectionID)
		b[2] = 1 // header
		b[3] = 0 // extension
	}), nil
}

func (*utpMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return (*utpMask)(nil).WrapPacketConnClient(raw, first, leaveSize, end)
}

// --- WechatVideo ---

type wechatMask struct{}

func newWechatMask() Udpmask { return &wechatMask{} }

func (*wechatMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	sn := uint32(rand.Uint32() & 0xFFFF)
	return newHeaderConn(raw, first, leaveSize, 13, func(b []byte) {
		sn++
		b[0] = 0xa1
		b[1] = 0x08
		binary.BigEndian.PutUint32(b[2:], sn)
		b[6] = 0x00
		b[7] = 0x10
		b[8] = 0x11
		b[9] = 0x18
		b[10] = 0x30
		b[11] = 0x22
		b[12] = 0x30
	}), nil
}

func (*wechatMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return (*wechatMask)(nil).WrapPacketConnClient(raw, first, leaveSize, end)
}

// --- DTLS ---

type dtlsMask struct{}

func newDTLSMask() Udpmask { return &dtlsMask{} }

func (*dtlsMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	epoch := uint16(rand.Uint32())
	length := uint16(17)
	var sequence uint32
	return newHeaderConn(raw, first, leaveSize, 13, func(b []byte) {
		b[0] = 23
		b[1] = 254
		b[2] = 253
		b[3] = byte(epoch >> 8)
		b[4] = byte(epoch)
		b[5] = 0
		b[6] = 0
		b[7] = byte(sequence >> 24)
		b[8] = byte(sequence >> 16)
		b[9] = byte(sequence >> 8)
		b[10] = byte(sequence)
		sequence++
		b[11] = byte(length >> 8)
		b[12] = byte(length)
		length += 17
		if length > 100 {
			length -= 50
		}
	}), nil
}

func (*dtlsMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return (*dtlsMask)(nil).WrapPacketConnClient(raw, first, leaveSize, end)
}

// --- WireGuard ---

type wireguardMask struct{}

func newWireGuardMask() Udpmask { return &wireguardMask{} }

func (*wireguardMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newHeaderConn(raw, first, leaveSize, 4, func(b []byte) {
		b[0] = 0x04
		b[1] = 0x00
		b[2] = 0x00
		b[3] = 0x00
	}), nil
}

func (*wireguardMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return (*wireguardMask)(nil).WrapPacketConnClient(raw, first, leaveSize, end)
}

// --- DNS ---

type dnsMask struct {
	domain string
}

func newDNSMask(domain string) Udpmask {
	return &dnsMask{domain: domain}
}

func (m *dnsMask) buildHeader() ([]byte, error) {
	var header []byte
	header = binary.BigEndian.AppendUint16(header, 0x0000) // Transaction ID
	header = binary.BigEndian.AppendUint16(header, 0x0100) // Flags: Standard query
	header = binary.BigEndian.AppendUint16(header, 0x0001) // Questions
	header = binary.BigEndian.AppendUint16(header, 0x0000) // Answer RRs
	header = binary.BigEndian.AppendUint16(header, 0x0000) // Authority RRs
	header = binary.BigEndian.AppendUint16(header, 0x0000) // Additional RRs

	// Pack domain name
	domain := m.domain + "."
	buf := make([]byte, 0x100)
	off := 0
	begin := 0
	for i := 0; i < len(domain); i++ {
		if domain[i] == '.' {
			labelLen := i - begin
			buf[off] = byte(labelLen)
			copy(buf[off+1:], domain[begin:i])
			off += 1 + labelLen
			begin = i + 1
		}
	}
	buf[off] = 0
	off++
	header = append(header, buf[:off]...)
	header = binary.BigEndian.AppendUint16(header, 0x0001) // Type: A
	header = binary.BigEndian.AppendUint16(header, 0x0001) // Class: IN
	return header, nil
}

func (m *dnsMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	header, err := m.buildHeader()
	if err != nil {
		return nil, err
	}
	headerLen := int32(len(header))
	return newHeaderConn(raw, first, leaveSize, headerLen, func(b []byte) {
		copy(b, header)
		binary.BigEndian.PutUint16(b[0:], uint16(rand.Uint32()))
	}), nil
}

func (m *dnsMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return m.WrapPacketConnClient(raw, first, leaveSize, end)
}
