package mkcp

import (
	"context"
	"io"
	"math/rand"
	"net"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/kcp"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var globalConv = uint32(rand.Uint32() & 0xFFFF)

func fetchInput(_ context.Context, input io.Reader, reader kcp.PacketReader, conn *kcp.Connection) {
	// Each UDP packet must be read and processed individually.
	// Do NOT use buf.ReadFrom here as it loops until buffer is full,
	// accumulating multiple packets and causing massive ACK latency.
	packet := make([]byte, 16*1024)
	for {
		n, err := input.Read(packet)
		if n > 0 {
			segments := reader.Read(packet[:n])
			if len(segments) > 0 {
				conn.Input(segments)
			}
		}
		if err != nil {
			return
		}
	}
}

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	config     *kcp.Config
	tlsConfig  tls.Config
	maskMgr    *UdpmaskManager
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayMKCPOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	config := &kcp.Config{
		MTU:              options.MTU,
		TTI:              options.TTI,
		UplinkCapacity:   options.UplinkCapacity,
		DownlinkCapacity: options.DownlinkCapacity,
		Congestion:       options.Congestion,
		WriteBufferSize:  options.WriteBufferSize,
		ReadBufferSize:   options.ReadBufferSize,
		Seed:             options.Seed,
	}

	var maskMgr *UdpmaskManager
	if len(options.Masks) > 0 {
		masks, err := buildMasks(options.Masks)
		if err != nil {
			return nil, err
		}
		maskMgr = NewUdpmaskManager(masks)
	}

	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		config:     config,
		tlsConfig:  tlsConfig,
		maskMgr:    maskMgr,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	udpConn, err := c.dialer.DialContext(ctx, N.NetworkUDP, c.serverAddr)
	if err != nil {
		return nil, err
	}

	// Always wrap as connPacketConn to ensure Write() is used instead of WriteTo()
	// for connected UDP sockets (WriteTo() fails on connected sockets in Go).
	var rawConn net.PacketConn = &connPacketConn{Conn: udpConn}

	if c.maskMgr != nil {
		rawConn, err = c.maskMgr.WrapPacketConnClient(rawConn)
		if err != nil {
			udpConn.Close()
			return nil, err
		}
	}

	reader := &kcp.KCPPacketReader{}
	conv := uint16(atomic.AddUint32(&globalConv, 1))

	// Wrap PacketConn as io.Writer for KCP
	pcWriter := &packetConnWriter{conn: rawConn, dest: udpConn.RemoteAddr()}

	// Use NewRawConnection which uses RawSegmentWriter (no SimpleAuthenticator)
	session := kcp.NewRawConnection(kcp.ConnMetadata{
		LocalAddr:    rawConn.LocalAddr(),
		RemoteAddr:   udpConn.RemoteAddr(),
		Conversation: conv,
	}, pcWriter, pcWriter, c.config)

	// Wrap PacketConn as io.Reader for fetchInput
	pcReader := &packetConnReader{conn: rawConn}
	go fetchInput(ctx, pcReader, reader, session)

	var iConn net.Conn = session

	if c.tlsConfig != nil {
		tlsConn, err := tls.ClientHandshake(ctx, iConn, c.tlsConfig)
		if err != nil {
			session.Close()
			return nil, err
		}
		iConn = tlsConn
	}

	return iConn, nil
}

func (c *Client) Close() error {
	return nil
}

// connPacketConn wraps a net.Conn as a net.PacketConn for UDP connections.
type connPacketConn struct {
	net.Conn
}

func (c *connPacketConn) ReadFrom(p []byte) (n int, addr net.Addr, err error) {
	n, err = c.Conn.Read(p)
	if err != nil {
		return 0, nil, err
	}
	return n, c.Conn.RemoteAddr(), nil
}

func (c *connPacketConn) WriteTo(p []byte, addr net.Addr) (n int, err error) {
	return c.Conn.Write(p)
}

// packetConnWriter wraps a net.PacketConn as an io.WriteCloser.
type packetConnWriter struct {
	conn net.PacketConn
	dest net.Addr
}

func (w *packetConnWriter) Write(p []byte) (int, error) {
	n, err := w.conn.WriteTo(p, w.dest)
	if err != nil {
		// Log error for debugging
		_ = err
	}
	return n, err
}

func (w *packetConnWriter) Close() error {
	return w.conn.Close()
}

// packetConnReader wraps a net.PacketConn as an io.Reader.
type packetConnReader struct {
	conn net.PacketConn
	buf  []byte
}

func (r *packetConnReader) Read(p []byte) (int, error) {
	if r.buf == nil {
		r.buf = make([]byte, 65536)
	}
	n, _, err := r.conn.ReadFrom(r.buf)
	if err != nil {
		return 0, err
	}
	copy(p, r.buf[:n])
	return n, nil
}
