package kcp

import (
	"io"
	"sync"
	"time"

	"github.com/sagernet/sing/common/buf"
)

type SegmentWriter interface {
	Write(seg Segment) error
}

type SimpleSegmentWriter struct {
	sync.Mutex
	buffer *buf.Buffer
	writer io.Writer
}

func NewSegmentWriter(writer io.Writer) SegmentWriter {
	return &SimpleSegmentWriter{
		writer: writer,
		buffer: buf.New(),
	}
}

func (w *SimpleSegmentWriter) Write(seg Segment) error {
	w.Lock()
	defer w.Unlock()

	w.buffer.Reset()
	rawBytes := w.buffer.Extend(int(seg.ByteSize()))
	seg.Serialize(rawBytes)

	_, err := w.writer.Write(w.buffer.Bytes())
	return err
}

type RetryableWriter struct {
	writer SegmentWriter
}

func NewRetryableWriter(writer SegmentWriter) SegmentWriter {
	return &RetryableWriter{writer: writer}
}

func (w *RetryableWriter) Write(seg Segment) error {
	var lastErr error
	for i := 0; i < 5; i++ {
		if err := w.writer.Write(seg); err == nil {
			return nil
		} else {
			lastErr = err
		}
		time.Sleep(100 * time.Millisecond)
	}
	return lastErr
}
