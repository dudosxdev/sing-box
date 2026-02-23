package v2rayxhttp

import (
	"io"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

// splitConn implements net.Conn over separate reader/writer streams
type splitConn struct {
	writer     io.WriteCloser
	reader     io.ReadCloser
	remoteAddr net.Addr
	localAddr  net.Addr
	onClose    func()
	closeOnce  sync.Once
}

func (c *splitConn) Write(b []byte) (int, error) {
	return c.writer.Write(b)
}

func (c *splitConn) Read(b []byte) (int, error) {
	return c.reader.Read(b)
}

func (c *splitConn) Close() error {
	if c.onClose != nil {
		c.closeOnce.Do(c.onClose)
	}
	err := c.writer.Close()
	err2 := c.reader.Close()
	if err != nil {
		return err
	}
	return err2
}

func (c *splitConn) LocalAddr() net.Addr {
	if c.localAddr != nil {
		return c.localAddr
	}
	return &net.TCPAddr{}
}

func (c *splitConn) RemoteAddr() net.Addr {
	if c.remoteAddr != nil {
		return c.remoteAddr
	}
	return &net.TCPAddr{}
}

func (c *splitConn) SetDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *splitConn) SetReadDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *splitConn) SetWriteDeadline(t time.Time) error {
	return os.ErrInvalid
}

func (c *splitConn) NeedAdditionalReadDeadline() bool {
	return true
}

// waitReadCloser blocks Read until the underlying ReadCloser is set or closed.
type waitReadCloser struct {
	wait chan struct{}
	io.ReadCloser
}

func newWaitReadCloser() *waitReadCloser {
	return &waitReadCloser{
		wait: make(chan struct{}),
	}
}

func (w *waitReadCloser) Set(rc io.ReadCloser) {
	w.ReadCloser = rc
	defer func() {
		if recover() != nil {
			rc.Close()
		}
	}()
	close(w.wait)
}

func (w *waitReadCloser) Read(b []byte) (int, error) {
	if w.ReadCloser == nil {
		if <-w.wait; w.ReadCloser == nil {
			return 0, io.ErrClosedPipe
		}
	}
	return w.ReadCloser.Read(b)
}

func (w *waitReadCloser) Close() error {
	if w.ReadCloser != nil {
		return w.ReadCloser.Close()
	}
	defer func() {
		if recover() != nil && w.ReadCloser != nil {
			w.ReadCloser.Close()
		}
	}()
	close(w.wait)
	return nil
}

// httpServerConn wraps http.ResponseWriter + io.Reader for the server side
type httpServerConn struct {
	sync.Mutex
	closed bool
	done   chan struct{}
	io.Reader
	http.ResponseWriter
}

func newHTTPServerConn(reader io.Reader, writer http.ResponseWriter) *httpServerConn {
	return &httpServerConn{
		done:           make(chan struct{}),
		Reader:         reader,
		ResponseWriter: writer,
	}
}

func (c *httpServerConn) Write(b []byte) (int, error) {
	c.Lock()
	defer c.Unlock()
	if c.closed {
		return 0, io.ErrClosedPipe
	}
	n, err := c.ResponseWriter.Write(b)
	if err == nil {
		if flusher, ok := c.ResponseWriter.(http.Flusher); ok {
			flusher.Flush()
		}
	}
	return n, err
}

func (c *httpServerConn) Close() error {
	c.Lock()
	defer c.Unlock()
	if !c.closed {
		c.closed = true
		close(c.done)
	}
	return nil
}

func (c *httpServerConn) Wait() <-chan struct{} {
	return c.done
}

// nopWriteCloser wraps an io.Writer to add a no-op Close method
type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }

// nopReadCloser wraps an io.Reader to add a no-op Close method
type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error { return nil }

// pipeCloseWriter wraps *io.PipeWriter as io.WriteCloser with Write method
type pipeWriteCloser struct {
	*io.PipeWriter
}

func (w *pipeWriteCloser) Write(b []byte) (int, error) {
	return w.PipeWriter.Write(b)
}

func (w *pipeWriteCloser) Close() error {
	return w.PipeWriter.Close()
}
