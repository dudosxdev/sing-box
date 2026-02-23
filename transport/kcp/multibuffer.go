package kcp

import (
	"io"

	"github.com/sagernet/sing/common/buf"
)

// MultiBuffer is a list of buffers, similar to Xray-core's buf.MultiBuffer.
type MultiBuffer []*buf.Buffer

// IsEmpty returns true if the MultiBuffer is empty.
func (mb MultiBuffer) IsEmpty() bool {
	for _, b := range mb {
		if !b.IsEmpty() {
			return false
		}
	}
	return true
}

// ReleaseMulti releases all buffers in the MultiBuffer.
func ReleaseMulti(mb MultiBuffer) {
	for _, b := range mb {
		b.Release()
	}
}

// SplitBytes splits bytes from the MultiBuffer into b, returning the remaining MultiBuffer and bytes read.
func SplitBytes(mb MultiBuffer, b []byte) (MultiBuffer, int) {
	totalBytes := 0
	endIndex := -1
	for i, buffer := range mb {
		pBuffer := len(b) - totalBytes
		if pBuffer <= 0 {
			endIndex = i
			break
		}
		if buffer.Len() <= pBuffer {
			totalBytes += copy(b[totalBytes:], buffer.Bytes())
			buffer.Release()
		} else {
			totalBytes += copy(b[totalBytes:], buffer.Bytes()[:pBuffer])
			buffer.Advance(pBuffer)
			endIndex = i
			break
		}
	}
	if endIndex == -1 {
		return mb[:0], totalBytes
	}
	return mb[endIndex:], totalBytes
}

// MultiBufferContainer is a ReadWriteCloser wrapper over MultiBuffer.
type MultiBufferContainer struct {
	MultiBuffer
}

func (c *MultiBufferContainer) Read(b []byte) (int, error) {
	if c.MultiBuffer.IsEmpty() {
		return 0, io.EOF
	}
	mb, nBytes := SplitBytes(c.MultiBuffer, b)
	c.MultiBuffer = mb
	return nBytes, nil
}

func (c *MultiBufferContainer) Close() error {
	ReleaseMulti(c.MultiBuffer)
	c.MultiBuffer = nil
	return nil
}
