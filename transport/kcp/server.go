package kcp

import (
	"context"
	"net"
	"os"
	"sync"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type ConnectionID struct {
	Remote string
	Port   int
	Conv   uint16
}

type Server struct {
	ctx       context.Context
	logger    logger.ContextLogger
	tlsConfig tls.ServerConfig
	handler   adapter.V2RayServerTransportHandler
	config    *Config
	reader    PacketReader

	sync.Mutex
	sessions   map[ConnectionID]*Connection
	udpConn    net.PacketConn
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayKCPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (adapter.V2RayServerTransport, error) {
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
	return &Server{
		ctx:       ctx,
		logger:    logger,
		tlsConfig: tlsConfig,
		handler:   handler,
		config:    config,
		reader:    &KCPPacketReader{},
		sessions:  make(map[ConnectionID]*Connection),
	}, nil
}

func (s *Server) Network() []string {
	return []string{N.NetworkUDP}
}

func (s *Server) Serve(listener net.Listener) error {
	return os.ErrInvalid
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	s.Lock()
	s.udpConn = listener
	s.Unlock()
	go s.handlePackets(listener)
	return nil
}

func (s *Server) handlePackets(conn net.PacketConn) {
	buf := make([]byte, 65536)
	for {
		n, addr, err := conn.ReadFrom(buf)
		if err != nil {
			if E.IsClosedOrCanceled(err) {
				return
			}
			s.logger.Error("kcp server read error: ", err)
			return
		}
		s.onReceive(buf[:n], addr, conn)
	}
}

func (s *Server) onReceive(payload []byte, src net.Addr, conn net.PacketConn) {
	segments := s.reader.Read(payload)
	if len(segments) == 0 {
		return
	}

	conv := segments[0].Conversation()
	cmd := segments[0].Command()

	udpAddr, ok := src.(*net.UDPAddr)
	if !ok {
		return
	}

	id := ConnectionID{
		Remote: udpAddr.IP.String(),
		Port:   udpAddr.Port,
		Conv:   conv,
	}

	s.Lock()
	defer s.Unlock()

	session, found := s.sessions[id]

	if !found {
		if cmd == CommandTerminate {
			return
		}
		writer := &serverWriter{
			id:     id,
			conn:   conn,
			dest:   src,
			server: s,
		}
		session = NewConnection(ConnMetadata{
			LocalAddr:    conn.LocalAddr(),
			RemoteAddr:   src,
			Conversation: conv,
		}, writer, writer, s.config)

		s.sessions[id] = session

		go func() {
			var netConn net.Conn = session
			if s.tlsConfig != nil {
				tlsConn, err := tls.ServerHandshake(s.ctx, session, s.tlsConfig)
				if err != nil {
					s.logger.Error("kcp tls handshake error: ", err)
					session.Close()
					return
				}
				netConn = tlsConn
			}
			s.handler.NewConnectionEx(s.ctx, netConn, M.SocksaddrFromNet(src), M.Socksaddr{}, nil)
		}()
	}
	session.Input(segments)
}

func (s *Server) removeSession(id ConnectionID) {
	s.Lock()
	delete(s.sessions, id)
	s.Unlock()
}

func (s *Server) Close() error {
	s.Lock()
	defer s.Unlock()

	if s.udpConn != nil {
		s.udpConn.Close()
	}

	for _, conn := range s.sessions {
		go conn.Terminate()
	}

	return nil
}

type serverWriter struct {
	id     ConnectionID
	conn   net.PacketConn
	dest   net.Addr
	server *Server
}

func (w *serverWriter) Write(payload []byte) (int, error) {
	return w.conn.WriteTo(payload, w.dest)
}

func (w *serverWriter) Close() error {
	w.server.removeSession(w.id)
	return nil
}
