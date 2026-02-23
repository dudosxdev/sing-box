package v2rayxhttp

import (
	"bytes"
	"context"
	"io"
	"math"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/tls"
	"github.com/sagernet/sing-box/option"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	sHTTP "github.com/sagernet/sing/protocol/http"

	"golang.org/x/net/http2"
)

var _ adapter.V2RayClientTransport = (*Client)(nil)

type Client struct {
	dialer     N.Dialer
	serverAddr M.Socksaddr
	tlsConfig  tls.Config
	config     *xhttpConfig
	options    option.V2RayXHTTPOptions
	requestURL url.URL
	http2      bool

	// xmux
	xmuxAccess  sync.Mutex
	xmuxManager *xmuxManager
}

func NewClient(ctx context.Context, dialer N.Dialer, serverAddr M.Socksaddr, options option.V2RayXHTTPOptions, tlsConfig tls.Config) (*Client, error) {
	cfg := newConfig(options)

	var requestURL url.URL
	if tlsConfig == nil {
		requestURL.Scheme = "http"
	} else {
		requestURL.Scheme = "https"
	}
	requestURL.Host = serverAddr.String()
	err := sHTTP.URLSetPath(&requestURL, cfg.path)
	if err != nil {
		return nil, E.Cause(err, "parse path")
	}
	if cfg.query != "" {
		requestURL.RawQuery = cfg.query
	}

	if cfg.host == "" {
		if tlsConfig != nil && tlsConfig.ServerName() != "" {
			cfg.host = tlsConfig.ServerName()
		} else {
			cfg.host = serverAddr.AddrString()
		}
	}

	isHTTP2 := false
	if tlsConfig != nil {
		isHTTP2 = true
		if len(tlsConfig.NextProtos()) == 0 {
			tlsConfig.SetNextProtos([]string{http2.NextProtoTLS})
		}
	}

	client := &Client{
		dialer:     dialer,
		serverAddr: serverAddr,
		tlsConfig:  tlsConfig,
		config:     cfg,
		options:    options,
		requestURL: requestURL,
		http2:      isHTTP2,
	}

	return client, nil
}

func (c *Client) createHTTPClient() *http.Client {
	var transport http.RoundTripper
	if c.tlsConfig != nil {
		tlsDialer := tls.NewDialer(c.dialer, c.tlsConfig)
		transport = &http2.Transport{
			DialTLSContext: func(ctx context.Context, network, addr string, cfg *tls.STDConfig) (net.Conn, error) {
				return tlsDialer.DialTLSContext(ctx, M.ParseSocksaddr(addr))
			},
			IdleConnTimeout: 90 * time.Second,
		}
	} else {
		transport = &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				return c.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
			},
			IdleConnTimeout:   90 * time.Second,
			DisableKeepAlives: true,
		}
	}
	return &http.Client{Transport: transport}
}

func (c *Client) getHTTPClient() (*http.Client, *xmuxClient) {
	if c.options.Xmux == nil {
		return c.createHTTPClient(), nil
	}

	c.xmuxAccess.Lock()
	defer c.xmuxAccess.Unlock()

	if c.xmuxManager == nil {
		c.xmuxManager = newXmuxManager(c.options.Xmux, func() *http.Client {
			return c.createHTTPClient()
		})
	}

	xc := c.xmuxManager.getClient()
	return xc.httpClient, xc
}

func (c *Client) DialContext(ctx context.Context) (net.Conn, error) {
	cfg := c.config
	mode := cfg.mode

	sessionID := ""
	if mode != "stream-one" {
		id, err := uuid.NewV4()
		if err != nil {
			return nil, E.Cause(err, "generate session ID")
		}
		sessionID = id.String()
	}

	httpClient, xmuxClient := c.getHTTPClient()

	reader, writer := io.Pipe()
	conn := &splitConn{
		writer: &pipeWriteCloser{writer},
		onClose: func() {
			if xmuxClient != nil {
				xmuxClient.openUsage.Add(-1)
			}
		},
	}
	if xmuxClient != nil {
		xmuxClient.openUsage.Add(1)
	}

	switch mode {
	case "stream-one":
		if xmuxClient != nil {
			xmuxClient.leftRequests.Add(-1)
		}
		return c.dialStreamOne(ctx, httpClient, conn, reader)
	case "stream-up":
		return c.dialStreamUp(ctx, httpClient, xmuxClient, conn, sessionID, reader)
	default: // packet-up
		return c.dialPacketUp(ctx, httpClient, xmuxClient, conn, sessionID, reader)
	}
}

func (c *Client) buildRequest(ctx context.Context, method string, sessionID string, seqStr string, body io.Reader) (*http.Request, error) {
	cfg := c.config
	reqURL := c.requestURL

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), body)
	if err != nil {
		return nil, err
	}
	req.Host = cfg.host
	req.Header = cfg.headers.Clone()

	// Apply meta (session/seq)
	cfg.applyMetaToRequest(req, sessionID, seqStr)

	// Apply padding
	cfg.applyXPaddingToRequest(req, reqURL.String())

	// Apply gRPC header for upload methods
	if method == cfg.uplinkHTTPMethod && !cfg.noGRPCHeader {
		req.Header.Set("Content-Type", "application/grpc")
	}

	return req, nil
}

func (c *Client) dialStreamOne(ctx context.Context, httpClient *http.Client, conn *splitConn, body *io.PipeReader) (net.Conn, error) {
	cfg := c.config
	req, err := c.buildRequest(ctx, cfg.uplinkHTTPMethod, "", "", body)
	if err != nil {
		return nil, err
	}

	wrc := newWaitReadCloser()
	conn.reader = wrc

	go func() {
		resp, err := httpClient.Do(req)
		if err != nil {
			wrc.Close()
			return
		}
		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			wrc.Close()
			return
		}
		wrc.Set(resp.Body)
	}()

	return conn, nil
}

func (c *Client) dialStreamUp(ctx context.Context, httpClient *http.Client, xmuxClient *xmuxClient, conn *splitConn, sessionID string, body *io.PipeReader) (net.Conn, error) {
	cfg := c.config

	// Download request (GET)
	downloadReq, err := c.buildRequest(ctx, "GET", sessionID, "", nil)
	if err != nil {
		return nil, err
	}

	wrc := newWaitReadCloser()
	conn.reader = wrc

	if xmuxClient != nil {
		xmuxClient.leftRequests.Add(-1)
	}
	go func() {
		resp, err := httpClient.Do(downloadReq)
		if err != nil {
			wrc.Close()
			return
		}
		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			wrc.Close()
			return
		}
		wrc.Set(resp.Body)
	}()

	// Upload request (POST/custom method)
	uploadReq, err := c.buildRequest(ctx, cfg.uplinkHTTPMethod, sessionID, "", body)
	if err != nil {
		return nil, err
	}

	if xmuxClient != nil {
		xmuxClient.leftRequests.Add(-1)
	}
	go func() {
		resp, err := httpClient.Do(uploadReq)
		if err != nil {
			return
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}()

	return conn, nil
}

func (c *Client) dialPacketUp(ctx context.Context, httpClient *http.Client, xmuxClient *xmuxClient, conn *splitConn, sessionID string, body *io.PipeReader) (net.Conn, error) {
	cfg := c.config

	// Download request (GET)
	downloadReq, err := c.buildRequest(ctx, "GET", sessionID, "", nil)
	if err != nil {
		return nil, err
	}

	wrc := newWaitReadCloser()
	conn.reader = wrc

	if xmuxClient != nil {
		xmuxClient.leftRequests.Add(-1)
	}
	go func() {
		resp, err := httpClient.Do(downloadReq)
		if err != nil {
			wrc.Close()
			return
		}
		if resp.StatusCode != http.StatusOK {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			wrc.Close()
			return
		}
		wrc.Set(resp.Body)
	}()

	// Upload goroutine
	go func() {
		var seq int64
		maxUploadSize := int(randRange(cfg.scMaxEachPostBytesFrom, cfg.scMaxEachPostBytesTo))
		buf := make([]byte, maxUploadSize)

		for {
			n, readErr := body.Read(buf)
			if n > 0 {
				seqStr := strconv.FormatInt(seq, 10)
				seq++

				chunk := make([]byte, n)
				copy(chunk, buf[:n])

				if xmuxClient != nil {
					xmuxClient.leftRequests.Add(-1)
				}

				var uploadBody io.Reader
				var contentLength int64

				if cfg.uplinkDataPlacement == PlacementBody {
					uploadBody = bytes.NewReader(chunk)
					contentLength = int64(n)
				}

				uploadReq, err := c.buildRequest(ctx, cfg.uplinkHTTPMethod, sessionID, seqStr, uploadBody)
				if err != nil {
					body.CloseWithError(err)
					return
				}
				if contentLength > 0 {
					uploadReq.ContentLength = contentLength
				}

				// Non-body data placement
				if cfg.uplinkDataPlacement != PlacementBody {
					cfg.encodeUplinkData(uploadReq, chunk)
				}

				resp, err := httpClient.Do(uploadReq)
				if err != nil {
					body.CloseWithError(err)
					return
				}
				io.Copy(io.Discard, resp.Body)
				resp.Body.Close()

				if resp.StatusCode != http.StatusOK {
					body.CloseWithError(E.New("xhttp: unexpected upload status: ", resp.Status))
					return
				}

				// Inter-post interval
				if cfg.scMinPostsIntervalMsFrom > 0 {
					interval := randRange(cfg.scMinPostsIntervalMsFrom, cfg.scMinPostsIntervalMsTo)
					time.Sleep(time.Duration(interval) * time.Millisecond)
				}
			}
			if readErr != nil {
				return
			}
		}
	}()

	return conn, nil
}

func (c *Client) Close() error {
	c.xmuxAccess.Lock()
	defer c.xmuxAccess.Unlock()
	c.xmuxManager = nil
	return nil
}

// xmux implementation

type xmuxClient struct {
	httpClient   *http.Client
	openUsage    atomic.Int32
	leftUsage    int32
	leftRequests atomic.Int32
	unreusableAt time.Time
}

type xmuxManager struct {
	config      *option.V2RayXHTTPXmuxConfig
	concurrency int32
	connections int32
	newFunc     func() *http.Client
	clients     []*xmuxClient
}

func newXmuxManager(config *option.V2RayXHTTPXmuxConfig, newFunc func() *http.Client) *xmuxManager {
	m := &xmuxManager{
		config:  config,
		newFunc: newFunc,
		clients: make([]*xmuxClient, 0),
	}
	if config.MaxConcurrency != nil {
		m.concurrency = randRange(config.MaxConcurrency.From, config.MaxConcurrency.To)
	}
	if config.MaxConnections != nil {
		m.connections = randRange(config.MaxConnections.From, config.MaxConnections.To)
	}
	return m
}

func (m *xmuxManager) newClient() *xmuxClient {
	xc := &xmuxClient{
		httpClient: m.newFunc(),
		leftUsage:  -1,
	}
	if m.config.CMaxReuseTimes != nil {
		if x := randRange(m.config.CMaxReuseTimes.From, m.config.CMaxReuseTimes.To); x > 0 {
			xc.leftUsage = x - 1
		}
	}
	xc.leftRequests.Store(math.MaxInt32)
	if m.config.HMaxRequestTimes != nil {
		if x := randRange(m.config.HMaxRequestTimes.From, m.config.HMaxRequestTimes.To); x > 0 {
			xc.leftRequests.Store(int32(x))
		}
	}
	if m.config.HMaxReusableSecs != nil {
		if x := randRange(m.config.HMaxReusableSecs.From, m.config.HMaxReusableSecs.To); x > 0 {
			xc.unreusableAt = time.Now().Add(time.Duration(x) * time.Second)
		}
	}
	m.clients = append(m.clients, xc)
	return xc
}

func (m *xmuxManager) getClient() *xmuxClient {
	// Prune dead clients
	for i := 0; i < len(m.clients); {
		xc := m.clients[i]
		if xc.leftUsage == 0 ||
			xc.leftRequests.Load() <= 0 ||
			(!xc.unreusableAt.IsZero() && time.Now().After(xc.unreusableAt)) {
			m.clients = append(m.clients[:i], m.clients[i+1:]...)
		} else {
			i++
		}
	}

	if len(m.clients) == 0 {
		return m.newClient()
	}

	if m.connections > 0 && int32(len(m.clients)) < m.connections {
		return m.newClient()
	}

	// Filter by concurrency
	var available []*xmuxClient
	if m.concurrency > 0 {
		for _, xc := range m.clients {
			if xc.openUsage.Load() < m.concurrency {
				available = append(available, xc)
			}
		}
	} else {
		available = m.clients
	}

	if len(available) == 0 {
		return m.newClient()
	}

	idx := int(randRange(0, int32(len(available)-1)))
	xc := available[idx]
	if xc.leftUsage > 0 {
		xc.leftUsage--
	}
	return xc
}

// Unused imports guard
var (
	_ = strings.HasSuffix
)
