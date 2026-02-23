package mkcp

import (
	"encoding/binary"
	"hash/fnv"
	"io"
	"net"
	"sync"
	"time"
)

// originalMask implements the "original" XOR-based obfuscation from Xray-core finalmask.
type originalMask struct{}

func newOriginalMask() Udpmask {
	return &originalMask{}
}

func (*originalMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newOriginalConn(raw, first, leaveSize), nil
}

func (*originalMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newOriginalConn(raw, first, leaveSize), nil
}

type originalConn struct {
	first     bool
	leaveSize int32
	conn      net.PacketConn

	readBuf    []byte
	readMutex  sync.Mutex
	writeBuf   []byte
	writeMutex sync.Mutex
}

func newOriginalConn(raw net.PacketConn, first bool, leaveSize int32) *originalConn {
	c := &originalConn{
		first:     first,
		leaveSize: leaveSize,
		conn:      raw,
	}
	if first {
		c.readBuf = make([]byte, 8192)
		c.writeBuf = make([]byte, 8192)
	}
	return c
}

func (c *originalConn) Size() int32 { return 6 }

func xorfwd(x []byte) {
	for i := 4; i < len(x); i++ {
		x[i] ^= x[i-4]
	}
}

func xorbkd(x []byte) {
	for i := len(x) - 1; i >= 4; i-- {
		x[i] ^= x[i-4]
	}
}

func originalSeal(dst, plain []byte) []byte {
	dst = append(dst, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint16(dst[4:], uint16(len(plain)))
	dst = append(dst, plain...)

	fnvHash := fnv.New32a()
	fnvHash.Write(dst[4:])
	binary.BigEndian.PutUint32(dst[:4], fnvHash.Sum32())

	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorfwd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}
	return dst
}

func originalOpen(dst []byte) ([]byte, error) {
	dstLen := len(dst)
	xtra := 4 - dstLen%4
	if xtra != 4 {
		dst = append(dst, make([]byte, xtra)...)
	}
	xorbkd(dst)
	if xtra != 4 {
		dst = dst[:dstLen]
	}

	fnvHash := fnv.New32a()
	fnvHash.Write(dst[4:])
	if binary.BigEndian.Uint32(dst[:4]) != fnvHash.Sum32() {
		return nil, errInvalidAuth
	}

	length := binary.BigEndian.Uint16(dst[4:6])
	if len(dst)-6 != int(length) {
		return nil, errInvalidAuth
	}

	return dst[6:], nil
}

var errInvalidAuth = &maskError{"invalid auth"}

type maskError struct{ msg string }

func (e *maskError) Error() string { return e.msg }

func (c *originalConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
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
		opened, err := originalOpen(c.readBuf[:n])
		if err != nil {
			c.readMutex.Unlock()
			return 0, addr, err
		}
		copy(p, opened)
		c.readMutex.Unlock()
		return n - int(c.Size()), addr, nil
	}

	n, addr, err = c.conn.ReadFrom(p)
	if err != nil {
		return n, addr, err
	}
	if n < int(c.Size()) {
		return 0, addr, io.ErrShortBuffer
	}
	opened, err := originalOpen(p[:n])
	if err != nil {
		return 0, addr, err
	}
	copy(p, opened)
	return n - int(c.Size()), addr, nil
}

func (c *originalConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.first {
		if c.leaveSize+c.Size()+int32(len(p)) > 8192 {
			return 0, &maskError{"too many masks"}
		}
		c.writeMutex.Lock()
		n = copy(c.writeBuf[c.leaveSize+c.Size():], p)
		n += int(c.leaveSize) + int(c.Size())
		plaintext := c.writeBuf[c.leaveSize+c.Size() : n]
		sealed := originalSeal(nil, plaintext)
		copy(c.writeBuf[c.leaveSize:], sealed)
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
	plaintext := p[c.leaveSize+c.Size():]
	sealed := originalSeal(nil, plaintext)
	copy(p[c.leaveSize:], sealed)
	return c.conn.WriteTo(p, addr)
}

func min32(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func (c *originalConn) Close() error                       { return c.conn.Close() }
func (c *originalConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *originalConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *originalConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *originalConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
