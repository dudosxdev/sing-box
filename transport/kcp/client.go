package kcp

import (
	"context"
	"io"
	"math/rand"
	"net"
	"sync/atomic"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var globalConv = uint32(rand.Uint32() & 0xFFFF)

func fetchInput(_ context.Context, input io.Reader, reader PacketReader, conn *Connection) {
	cache := make(chan *buf.Buffer, 1024)
	go func() {
		for {
			payload := buf.New()
			// payload.ReadFrom calls input.Read(b.FreeBytes()).
			// For UDP (PacketConn), Read reads a single packet.
			// However, if 'input' is a net.Conn (like UDPConn), Read behaves similarly.
			// We need to ensure we are actually reading data.
			n, err := payload.ReadFrom(input)
			if err != nil {
				// common.Error(common.Error(err).Base(io.ErrUnexpectedEOF))
				payload.Release()
				close(cache)
				return
			}
			if n > 0 {
				select {
				case cache <- payload:
				default:
					payload.Release()
				}
			} else {
				payload.Release()
			}
		}
	}()

	for payload := range cache {
		segments := reader.Read(payload.Bytes())
		payload.Release()
		if len(segments) > 0 {
			conn.Input(segments)
		}
	}
}

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	ctx        context.Context
	dialer     N.Dialer
	serverAddr M.Socksaddr
	config     *Config
	tlsConfig  tls.Config
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayKCPOptions, tlsConfig tls.Config) (adapter.V2RayClientTransport, error) {
	config := &Config{
		MTU:              options.MTU,
		TTI:              options.TTI,
		UplinkCapacity:   options.UplinkCapacity,
		DownlinkCapacity: options.DownlinkCapacity,
		Congestion:       options.Congestion,
		WriteBufferSize:  options.WriteBufferSize,
		ReadBufferSize:   options.ReadBufferSize,
		Seed:             options.Seed,
	}
	return &Client{
		ctx:        ctx,
		dialer:     dialer,
		serverAddr: serverAddr,
		config:     config,
		tlsConfig:  tlsConfig,
	}, nil
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	udpConn, err := c.dialer.DialContext(ctx, N.NetworkUDP, c.serverAddr)
	if err != nil {
		return nil, err
	}

	reader := &KCPPacketReader{}
	conv := uint16(atomic.AddUint32(&globalConv, 1))
	session := NewConnection(ConnMetadata{
		LocalAddr:    udpConn.LocalAddr(),
		RemoteAddr:   udpConn.RemoteAddr(),
		Conversation: conv,
	}, udpConn, udpConn, c.config)

	go fetchInput(ctx, udpConn, reader, session)

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
