package kcp

import (
	"encoding/binary"
	"hash/fnv"
)

// SimpleAuthenticator is the legacy KCP obfuscation used by Xray-core.
// It wraps data with FNV32a hash + length + XOR forward.
type SimpleAuthenticator struct{}

func (*SimpleAuthenticator) NonceSize() int { return 0 }
func (*SimpleAuthenticator) Overhead() int  { return 6 }

func (a *SimpleAuthenticator) Seal(plain []byte) []byte {
	dst := make([]byte, 0, 6+len(plain))
	dst = append(dst, 0, 0, 0, 0, 0, 0)
	binary.BigEndian.PutUint16(dst[4:], uint16(len(plain)))
	dst = append(dst, plain...)

	fnvHash := fnv.New32a()
	fnvHash.Write(dst[4:])
	result := fnvHash.Sum(nil)
	copy(dst[:4], result)

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

func (a *SimpleAuthenticator) Open(cipherText []byte) ([]byte, error) {
	dst := make([]byte, len(cipherText))
	copy(dst, cipherText)

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

var errInvalidAuth = &authError{"invalid auth"}

type authError struct{ msg string }

func (e *authError) Error() string { return e.msg }

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
