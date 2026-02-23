package mkcp

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"io"
	"net"
	"sync"
	"time"
)

type aes128gcmMask struct {
	password string
}

func newAes128GcmMask(password string) Udpmask {
	return &aes128gcmMask{password: password}
}

func (m *aes128gcmMask) WrapPacketConnClient(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newAes128GcmConn(raw, m.password, first, leaveSize)
}

func (m *aes128gcmMask) WrapPacketConnServer(raw net.PacketConn, first bool, leaveSize int32, end bool) (net.PacketConn, error) {
	return newAes128GcmConn(raw, m.password, first, leaveSize)
}

type aes128gcmConn struct {
	first     bool
	leaveSize int32
	conn      net.PacketConn
	aead      cipher.AEAD

	readBuf    []byte
	readMutex  sync.Mutex
	writeBuf   []byte
	writeMutex sync.Mutex
}

func newAes128GcmConn(raw net.PacketConn, password string, first bool, leaveSize int32) (*aes128gcmConn, error) {
	hashedPsk := sha256.Sum256([]byte(password))
	block, err := aes.NewCipher(hashedPsk[:16])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	c := &aes128gcmConn{
		first:     first,
		leaveSize: leaveSize,
		conn:      raw,
		aead:      aead,
	}
	if first {
		c.readBuf = make([]byte, 8192)
		c.writeBuf = make([]byte, 8192)
	}
	return c, nil
}

func (c *aes128gcmConn) Size() int32 {
	return int32(c.aead.NonceSize()) + int32(c.aead.Overhead())
}

func (c *aes128gcmConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
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
		nonceSize := c.aead.NonceSize()
		nonce := c.readBuf[:nonceSize]
		ciphertext := c.readBuf[nonceSize:n]
		_, err = c.aead.Open(p[:0], nonce, ciphertext, nil)
		if err != nil {
			c.readMutex.Unlock()
			return 0, addr, err
		}
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
	nonceSize := c.aead.NonceSize()
	nonce := p[:nonceSize]
	ciphertext := p[nonceSize:n]
	_, err = c.aead.Open(ciphertext[:0], nonce, ciphertext, nil)
	if err != nil {
		return 0, addr, err
	}
	copy(p, p[nonceSize:n-c.aead.Overhead()])
	return n - int(c.Size()), addr, nil
}

func (c *aes128gcmConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	if c.first {
		if c.leaveSize+c.Size()+int32(len(p)) > 8192 {
			return 0, &maskError{"too many masks"}
		}
		c.writeMutex.Lock()
		nonceSize := c.aead.NonceSize()
		n = copy(c.writeBuf[c.leaveSize+int32(nonceSize):], p)
		n += int(c.leaveSize) + int(c.Size())
		nonce := c.writeBuf[c.leaveSize : c.leaveSize+int32(nonceSize)]
		if _, err = rand.Read(nonce); err != nil {
			c.writeMutex.Unlock()
			return 0, err
		}
		plaintext := c.writeBuf[c.leaveSize+int32(nonceSize) : n-c.aead.Overhead()]
		c.aead.Seal(plaintext[:0], nonce, plaintext, nil)
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

	nonceSize := c.aead.NonceSize()
	nonce := p[c.leaveSize : c.leaveSize+int32(nonceSize)]
	if _, err = rand.Read(nonce); err != nil {
		return 0, err
	}
	copy(p[c.leaveSize+int32(nonceSize):], p[c.leaveSize+c.Size():])
	plaintext := p[c.leaveSize+int32(nonceSize) : len(p)-c.aead.Overhead()]
	c.aead.Seal(plaintext[:0], nonce, plaintext, nil)
	return c.conn.WriteTo(p, addr)
}

func (c *aes128gcmConn) Close() error                       { return c.conn.Close() }
func (c *aes128gcmConn) LocalAddr() net.Addr                { return c.conn.LocalAddr() }
func (c *aes128gcmConn) SetDeadline(t time.Time) error      { return c.conn.SetDeadline(t) }
func (c *aes128gcmConn) SetReadDeadline(t time.Time) error  { return c.conn.SetReadDeadline(t) }
func (c *aes128gcmConn) SetWriteDeadline(t time.Time) error { return c.conn.SetWriteDeadline(t) }
