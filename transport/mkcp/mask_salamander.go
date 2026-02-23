package mkcp

import (
	"fmt"
	"io"
	"math/rand"
	"net"
	"sync"
	"time"

	"golang.org/x/crypto/blake2b"
)

const (
	smPSKMinLen = 4
	smSaltLen   = 8
	smKeyLen    = blake2b.Size256
)

var errPSKTooShort = fmt.Errorf("PSK must be at least %d bytes", smPSKMinLen)

type salamanderObfuscator struct {
	psk []byte
	rng *rand.Rand
	mu  sync.Mutex
}

func newSalamanderObfuscator(psk []byte) (*salamanderObfuscator, error) {
	if len(psk) < smPSKMinLen {
		return nil, errPSKTooShort
	}
	return &salamanderObfuscator{
		psk: psk,
		rng: rand.New(rand.NewSource(time.Now().UnixNano())),
	}, nil
}

func (o *salamanderObfuscator) obfuscate(in, out []byte) int {
	outLen := len(in) + smSaltLen
	if len(out) < outLen {
		return 0
	}
	o.mu.Lock()
	_, _ = o.rng.Read(out[:smSaltLen])
	o.mu.Unlock()
	key := o.key(out[:smSaltLen])
	for i, c := range in {
		out[i+smSaltLen] = c ^ key[i%smKeyLen]
	}
	return outLen
}

func (o *salamanderObfuscator) deobfuscate(in, out []byte) int {
	outLen := len(in) - smSaltLen
	if outLen <= 0 || len(out) < outLen {
		return 0
	}
	key := o.key(in[:smSaltLen])
	for i, c := range in[smSaltLen:] {
		out[i] = c ^ key[i%smKeyLen]
	}
	return outLen
}

func (o *salamanderObfuscator) key(salt []byte) [smKeyLen]byte {
	return blake2b.Sum256(append(o.psk, salt...))
}

type salamanderMask struct {
	password string
}

func newSalamanderMask(password string) Udpmask {
	return &salamanderMask{password: password}
}

func (m *salamanderMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newSalamanderConn(raw, m.password, first, leaveSize)
}

func (m *salamanderMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newSalamanderConn(raw, m.password, first, leaveSize)
}

type salamanderConn struct {
	first     bool
	leaveSize int32
	conn      net.PacketConn
	obfs      *salamanderObfuscator

	readBuf    []byte
	readMutex  sync.Mutex
	writeBuf   []byte
	writeMutex sync.Mutex
}

func newSalamanderConn(raw net.PacketConn, password string, first bool, leaveSize int32) (*salamanderConn, error) {
	ob, err := newSalamanderObfuscator([]byte(password))
	if err != nil {
		return nil, err
	}
	c := &salamanderConn{
		first:     first,
		leaveSize: leaveSize,
		conn:      raw,
		obfs:      ob,
	}
	if first {
		c.readBuf = make([]byte, 8192)
		c.writeBuf = make([]byte, 8192)
	}
	return c, nil
}

func (c *salamanderConn) Size() int32 { return smSaltLen }

func (c *salamanderConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
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
		c.obfs.deobfuscate(c.readBuf[:n], p)
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
	c.obfs.deobfuscate(p[:n], p)
	return n - int(c.Size()), addr, err
}

func (c *salamanderConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.first {
		if c.leaveSize+c.Size()+int32(len(p)) > 8192 {
			return 0, &maskError{"too many masks"}
		}
		c.writeMutex.Lock()
		n = copy(c.writeBuf[c.leaveSize+c.Size():], p)
		n += int(c.leaveSize) + int(c.Size())
		c.obfs.obfuscate(c.writeBuf[c.leaveSize+c.Size():n], c.writeBuf[c.leaveSize:n])
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
	c.obfs.obfuscate(p[c.leaveSize+c.Size():], p[c.leaveSize:])
	return c.conn.WriteTo(p, addr)
}

func (c *salamanderConn) Close() error                       { return c.conn.Close() }
func (c *salamanderConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *salamanderConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *salamanderConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *salamanderConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
