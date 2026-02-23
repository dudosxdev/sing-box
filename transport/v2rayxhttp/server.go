package v2rayxhttp

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/transport/v2rayhttp"
	"github.com/sagernet/sing/common"
	E "github.com/sagernet/sing/common/exceptions"
	"github.com/sagernet/sing/common/logger"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	aTLS "github.com/sagernet/sing/common/tls"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

var _ adapter.V2RayServerTransport = (*Server)(nil)

type Server struct {
	ctx        context.Context
	logger     logger.ContextLogger
	tlsConfig  tls.ServerConfig
	handler    adapter.V2RayServerTransportHandler
	httpServer *http.Server
	h2Server   *http2.Server
	h2cHandler http.Handler
	config     *xhttpConfig

	// Session management
	sessionMu sync.Mutex
	sessions  sync.Map
}

type xhttpSession struct {
	uploadQueue      *uploadQueue
	isFullyConnected chan struct{} // closed when GET request connects
}

func NewServer(ctx context.Context, logger logger.ContextLogger, options option.V2RayXHTTPOptions, tlsConfig tls.ServerConfig, handler adapter.V2RayServerTransportHandler) (*Server, error) {
	cfg := newConfig(options)

	server := &Server{
		ctx:       ctx,
		logger:    logger,
		tlsConfig: tlsConfig,
		handler:   handler,
		config:    cfg,
		h2Server:  &http2.Server{},
	}

	server.httpServer = &http.Server{
		Handler:           server,
		ReadHeaderTimeout: C.TCPTimeout,
		MaxHeaderBytes:    http.DefaultMaxHeaderBytes,
		BaseContext: func(net.Listener) context.Context {
			return ctx
		},
		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			return log.ContextWithNewID(ctx)
		},
	}
	server.h2cHandler = h2c.NewHandler(server, server.h2Server)

	return server, nil
}

func (s *Server) upsertSession(sessionID string) *xhttpSession {
	if sessionAny, ok := s.sessions.Load(sessionID); ok {
		return sessionAny.(*xhttpSession)
	}

	s.sessionMu.Lock()
	defer s.sessionMu.Unlock()

	if sessionAny, ok := s.sessions.Load(sessionID); ok {
		return sessionAny.(*xhttpSession)
	}

	session := &xhttpSession{
		uploadQueue:      newUploadQueue(s.config.scMaxBufferedPosts),
		isFullyConnected: make(chan struct{}),
	}
	s.sessions.Store(sessionID, session)

	shouldReap := make(chan struct{})
	go func() {
		time.Sleep(30 * time.Second)
		close(shouldReap)
	}()
	go func() {
		select {
		case <-shouldReap:
			s.sessions.Delete(sessionID)
			session.uploadQueue.Close()
		case <-session.isFullyConnected:
		}
	}()

	return session
}

func (s *Server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	cfg := s.config

	// Handle h2c upgrade
	if request.Method == "PRI" && len(request.Header) == 0 && request.URL.Path == "*" && request.Proto == "HTTP/2.0" {
		s.h2cHandler.ServeHTTP(writer, request)
		return
	}

	// Validate host
	if len(cfg.host) > 0 && request.Host != cfg.host {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.New("bad host: ", request.Host))
		return
	}

	// Validate path prefix
	if !strings.HasPrefix(request.URL.Path, cfg.path) {
		s.invalidRequest(writer, request, http.StatusNotFound, E.New("bad path: ", request.URL.Path))
		return
	}

	// Write CORS response headers (matching Xray)
	writer.Header().Set("Access-Control-Allow-Origin", "*")
	writer.Header().Set("Access-Control-Allow-Methods", "*")

	// Apply response padding
	cfg.applyXPaddingToResponseHeader(writer.Header())

	// Validate request padding
	paddingValue := cfg.extractXPaddingFromRequest(request)
	if paddingValue != "" && !cfg.validatePadding(paddingValue) {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.New("invalid padding"))
		return
	}

	// Extract session ID and seq
	sessionID, seqStr := cfg.extractMetaFromRequest(request, cfg.path)

	// Check mode restrictions
	if sessionID == "" && cfg.mode != "" && cfg.mode != "auto" && cfg.mode != "stream-one" && cfg.mode != "stream-up" {
		s.invalidRequest(writer, request, http.StatusBadRequest, E.New("stream-one mode not allowed"))
		return
	}

	// Determine remote address
	remoteAddr := sHTTP.SourceAddress(request)

	var currentSession *xhttpSession
	if sessionID != "" {
		currentSession = s.upsertSession(sessionID)
	}

	isUplink := cfg.isUplinkRequest(request)

	if isUplink && sessionID != "" {
		// stream-up or packet-up
		if seqStr == "" {
			// stream-up: push the request body reader directly
			if cfg.mode != "" && cfg.mode != "auto" && cfg.mode != "stream-up" {
				s.invalidRequest(writer, request, http.StatusBadRequest, E.New("stream-up mode not allowed"))
				return
			}

			httpSC := newHTTPServerConn(request.Body, writer)
			err := currentSession.uploadQueue.Push(Packet{Reader: httpSC})
			if err != nil {
				s.invalidRequest(writer, request, http.StatusConflict, E.Cause(err, "push stream reader"))
				return
			}

			writer.Header().Set("X-Accel-Buffering", "no")
			writer.Header().Set("Cache-Control", "no-store")
			writer.WriteHeader(http.StatusOK)

			// Stream-up server keep-alive response (scStreamUpServerSecs)
			referrer := request.Header.Get("Referer")
			if referrer != "" && cfg.scStreamUpServerSecsTo > 0 {
				go func() {
					for {
						paddingLen := randRange(cfg.xPaddingBytesFrom, cfg.xPaddingBytesTo)
						_, err := httpSC.Write(bytes.Repeat([]byte{'X'}, int(paddingLen)))
						if err != nil {
							break
						}
						interval := randRange(cfg.scStreamUpServerSecsFrom, cfg.scStreamUpServerSecsTo)
						time.Sleep(time.Duration(interval) * time.Second)
					}
				}()
			}

			select {
			case <-request.Context().Done():
			case <-httpSC.Wait():
			}
			httpSC.Close()
			return
		}

		// packet-up: read payload and push with seq number
		if cfg.mode != "" && cfg.mode != "auto" && cfg.mode != "packet-up" {
			s.invalidRequest(writer, request, http.StatusBadRequest, E.New("packet-up mode not allowed"))
			return
		}

		var payload []byte
		var err error

		if cfg.uplinkDataPlacement != PlacementBody {
			// Extract data from headers/cookies
			payload, err = cfg.decodeUplinkData(request)
		} else {
			maxSize := int64(cfg.scMaxEachPostBytesTo) + 1
			payload, err = io.ReadAll(io.LimitReader(request.Body, maxSize))
		}

		if err != nil {
			s.invalidRequest(writer, request, http.StatusInternalServerError, E.Cause(err, "read upload"))
			return
		}

		if int32(len(payload)) > cfg.scMaxEachPostBytesTo {
			s.invalidRequest(writer, request, http.StatusRequestEntityTooLarge, E.New("upload too large"))
			return
		}

		seq, err := strconv.ParseUint(seqStr, 10, 64)
		if err != nil {
			s.invalidRequest(writer, request, http.StatusBadRequest, E.Cause(err, "parse seq"))
			return
		}

		err = currentSession.uploadQueue.Push(Packet{Payload: payload, Seq: seq})
		if err != nil {
			s.invalidRequest(writer, request, http.StatusInternalServerError, E.Cause(err, "push packet"))
			return
		}

		writer.WriteHeader(http.StatusOK)
	} else if request.Method == "GET" || sessionID == "" {
		// stream-down or stream-one
		if sessionID != "" {
			close(currentSession.isFullyConnected)
			defer s.sessions.Delete(sessionID)
		}

		writer.Header().Set("X-Accel-Buffering", "no")
		writer.Header().Set("Cache-Control", "no-store")
		if !cfg.noSSEHeader {
			writer.Header().Set("Content-Type", "text/event-stream")
		}

		writer.WriteHeader(http.StatusOK)
		if flusher, ok := writer.(http.Flusher); ok {
			flusher.Flush()
		}

		httpSC := newHTTPServerConn(request.Body, writer)

		var connReader io.ReadCloser
		if sessionID != "" {
			connReader = nopReadCloser{currentSession.uploadQueue}
		} else {
			connReader = httpSC
		}

		conn := &splitConn{
			writer:     httpSC,
			reader:     connReader,
			remoteAddr: remoteAddr.TCPAddr(),
		}

		s.handler.NewConnectionEx(v2rayhttp.DupContext(request.Context()), conn, remoteAddr, M.Socksaddr{}, nil)

		select {
		case <-request.Context().Done():
		case <-httpSC.Wait():
		}
		conn.Close()
	} else {
		s.invalidRequest(writer, request, http.StatusMethodNotAllowed, E.New("unsupported method: ", request.Method))
	}
}

func (s *Server) invalidRequest(writer http.ResponseWriter, request *http.Request, statusCode int, err error) {
	if statusCode > 0 {
		writer.WriteHeader(statusCode)
	}
	s.logger.ErrorContext(request.Context(), E.Cause(err, "process connection from ", request.RemoteAddr))
}

func (s *Server) Network() []string {
	return []string{N.NetworkTCP}
}

func (s *Server) Serve(listener net.Listener) error {
	if s.tlsConfig != nil {
		if len(s.tlsConfig.NextProtos()) == 0 {
			s.tlsConfig.SetNextProtos([]string{http2.NextProtoTLS, "http/1.1"})
		} else if !containsStr(s.tlsConfig.NextProtos(), http2.NextProtoTLS) {
			s.tlsConfig.SetNextProtos(append([]string{http2.NextProtoTLS}, s.tlsConfig.NextProtos()...))
		}
		listener = aTLS.NewListener(listener, s.tlsConfig)
	}
	return s.httpServer.Serve(listener)
}

func (s *Server) ServePacket(listener net.PacketConn) error {
	return os.ErrInvalid
}

func (s *Server) Close() error {
	return common.Close(common.PtrOrNil(s.httpServer))
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}
