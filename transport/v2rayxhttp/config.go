package v2rayxhttp

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/sagernet/sing-box/option"
	"golang.org/x/net/http2/hpack"
)

// Placement constants matching Xray
const (
	PlacementQueryInHeader = "queryInHeader"
	PlacementCookie        = "cookie"
	PlacementHeader        = "header"
	PlacementQuery         = "query"
	PlacementPath          = "path"
	PlacementBody          = "body"
)

// Padding methods
const (
	PaddingMethodRepeatX  = "repeat-x"
	PaddingMethodTokenish = "tokenish"
)

const defaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// xhttpConfig holds normalized configuration derived from options
type xhttpConfig struct {
	host    string
	path    string
	query   string
	mode    string
	headers http.Header

	// Padding
	xPaddingBytesFrom int32
	xPaddingBytesTo   int32
	xPaddingObfsMode  bool
	xPaddingKey       string
	xPaddingHeader    string
	xPaddingPlacement string
	xPaddingMethod    string

	noGRPCHeader bool
	noSSEHeader  bool

	// Upload settings
	uplinkHTTPMethod    string
	uplinkDataPlacement string
	uplinkDataKey       string
	uplinkChunkSize     int

	// Session/seq placement
	sessionPlacement string
	sessionKey       string
	seqPlacement     string
	seqKey           string

	// Packet-up
	scMaxEachPostBytesFrom   int32
	scMaxEachPostBytesTo     int32
	scMinPostsIntervalMsFrom int32
	scMinPostsIntervalMsTo   int32
	scMaxBufferedPosts       int

	// Stream-up server secs
	scStreamUpServerSecsFrom int32
	scStreamUpServerSecsTo   int32
}

func newConfig(options option.V2RayXHTTPOptions) *xhttpConfig {
	c := &xhttpConfig{
		host:         options.Host,
		noGRPCHeader: options.NoGRPCHeader,
		noSSEHeader:  options.NoSSEHeader,
	}

	// Normalize path and query
	pathAndQuery := strings.SplitN(options.Path, "?", 2)
	path := pathAndQuery[0]
	if path == "" || path[0] != '/' {
		path = "/" + path
	}
	if path[len(path)-1] != '/' {
		path = path + "/"
	}
	c.path = path
	if len(pathAndQuery) > 1 {
		c.query = pathAndQuery[1]
	}

	// Mode
	c.mode = options.Mode
	if c.mode == "" || c.mode == "auto" {
		c.mode = "packet-up"
	}

	// Headers
	c.headers = make(http.Header)
	for key, values := range options.Headers.Build() {
		for _, value := range values {
			c.headers.Add(key, value)
		}
	}
	if c.headers.Get("User-Agent") == "" {
		c.headers.Set("User-Agent", defaultUserAgent)
	}

	// Padding
	if options.XPaddingBytes != nil && options.XPaddingBytes.To > 0 {
		c.xPaddingBytesFrom = options.XPaddingBytes.From
		c.xPaddingBytesTo = options.XPaddingBytes.To
	} else {
		c.xPaddingBytesFrom = 100
		c.xPaddingBytesTo = 1000
	}
	c.xPaddingObfsMode = options.XPaddingObfsMode
	c.xPaddingKey = options.XPaddingKey
	c.xPaddingHeader = options.XPaddingHeader
	c.xPaddingPlacement = options.XPaddingPlacement
	c.xPaddingMethod = options.XPaddingMethod

	// Upload method
	c.uplinkHTTPMethod = options.UplinkHTTPMethod
	if c.uplinkHTTPMethod == "" {
		c.uplinkHTTPMethod = "POST"
	}

	// Uplink data placement
	c.uplinkDataPlacement = options.UplinkDataPlacement
	if c.uplinkDataPlacement == "" {
		c.uplinkDataPlacement = PlacementBody
	}
	c.uplinkDataKey = options.UplinkDataKey
	c.uplinkChunkSize = int(options.UplinkChunkSize)
	if c.uplinkChunkSize == 0 {
		c.uplinkChunkSize = 1000000
	}

	// Session/seq placement
	c.sessionPlacement = options.SessionPlacement
	if c.sessionPlacement == "" {
		c.sessionPlacement = PlacementPath
	}
	c.seqPlacement = options.SeqPlacement
	if c.seqPlacement == "" {
		c.seqPlacement = PlacementPath
	}

	// Session/seq keys
	c.sessionKey = options.SessionKey
	if c.sessionKey == "" {
		switch c.sessionPlacement {
		case PlacementHeader:
			c.sessionKey = "X-Session"
		case PlacementCookie, PlacementQuery:
			c.sessionKey = "x_session"
		}
	}
	c.seqKey = options.SeqKey
	if c.seqKey == "" {
		switch c.seqPlacement {
		case PlacementHeader:
			c.seqKey = "X-Seq"
		case PlacementCookie, PlacementQuery:
			c.seqKey = "x_seq"
		}
	}

	// Packet-up settings
	if options.ScMaxEachPostBytes != nil && options.ScMaxEachPostBytes.To > 0 {
		c.scMaxEachPostBytesFrom = options.ScMaxEachPostBytes.From
		c.scMaxEachPostBytesTo = options.ScMaxEachPostBytes.To
	} else {
		c.scMaxEachPostBytesFrom = 1000000
		c.scMaxEachPostBytesTo = 1000000
	}
	if options.ScMinPostsIntervalMs != nil && options.ScMinPostsIntervalMs.To > 0 {
		c.scMinPostsIntervalMsFrom = options.ScMinPostsIntervalMs.From
		c.scMinPostsIntervalMsTo = options.ScMinPostsIntervalMs.To
	} else {
		c.scMinPostsIntervalMsFrom = 30
		c.scMinPostsIntervalMsTo = 30
	}
	c.scMaxBufferedPosts = options.ScMaxBufferedPosts
	if c.scMaxBufferedPosts == 0 {
		c.scMaxBufferedPosts = 30
	}

	// Stream-up server secs
	if options.ScStreamUpServerSecs != nil && options.ScStreamUpServerSecs.To > 0 {
		c.scStreamUpServerSecsFrom = options.ScStreamUpServerSecs.From
		c.scStreamUpServerSecsTo = options.ScStreamUpServerSecs.To
	} else {
		c.scStreamUpServerSecsFrom = 20
		c.scStreamUpServerSecsTo = 80
	}

	return c
}

// applyMetaToRequest adds session ID and seq to the HTTP request
func (c *xhttpConfig) applyMetaToRequest(req *http.Request, sessionID string, seqStr string) {
	if sessionID != "" {
		switch c.sessionPlacement {
		case PlacementPath:
			req.URL.Path = appendToPath(req.URL.Path, sessionID)
		case PlacementQuery:
			q := req.URL.Query()
			q.Set(c.sessionKey, sessionID)
			req.URL.RawQuery = q.Encode()
		case PlacementHeader:
			req.Header.Set(c.sessionKey, sessionID)
		case PlacementCookie:
			req.AddCookie(&http.Cookie{Name: c.sessionKey, Value: sessionID})
		}
	}
	if seqStr != "" {
		switch c.seqPlacement {
		case PlacementPath:
			req.URL.Path = appendToPath(req.URL.Path, seqStr)
		case PlacementQuery:
			q := req.URL.Query()
			q.Set(c.seqKey, seqStr)
			req.URL.RawQuery = q.Encode()
		case PlacementHeader:
			req.Header.Set(c.seqKey, seqStr)
		case PlacementCookie:
			req.AddCookie(&http.Cookie{Name: c.seqKey, Value: seqStr})
		}
	}
}

// extractMetaFromRequest extracts session ID and seq from the HTTP request
func (c *xhttpConfig) extractMetaFromRequest(req *http.Request, basePath string) (sessionID string, seqStr string) {
	if c.sessionPlacement == PlacementPath && c.seqPlacement == PlacementPath {
		subpath := strings.Split(req.URL.Path[len(basePath):], "/")
		if len(subpath) > 0 {
			sessionID = subpath[0]
		}
		if len(subpath) > 1 {
			seqStr = subpath[1]
		}
		return
	}

	switch c.sessionPlacement {
	case PlacementQuery:
		sessionID = req.URL.Query().Get(c.sessionKey)
	case PlacementHeader:
		sessionID = req.Header.Get(c.sessionKey)
	case PlacementCookie:
		if cookie, e := req.Cookie(c.sessionKey); e == nil {
			sessionID = cookie.Value
		}
	}

	switch c.seqPlacement {
	case PlacementQuery:
		seqStr = req.URL.Query().Get(c.seqKey)
	case PlacementHeader:
		seqStr = req.Header.Get(c.seqKey)
	case PlacementCookie:
		if cookie, e := req.Cookie(c.seqKey); e == nil {
			seqStr = cookie.Value
		}
	}
	return
}

// applyXPaddingToRequest applies padding to an outgoing HTTP request
func (c *xhttpConfig) applyXPaddingToRequest(req *http.Request, rawURL string) {
	length := randRange(c.xPaddingBytesFrom, c.xPaddingBytesTo)
	if length <= 0 {
		return
	}

	paddingValue := generatePadding(c.xPaddingMethod, int(length))

	if c.xPaddingObfsMode {
		switch c.xPaddingPlacement {
		case PlacementHeader:
			req.Header.Set(c.xPaddingHeader, paddingValue)
		case PlacementQueryInHeader:
			u, err := url.Parse(rawURL)
			if err != nil {
				return
			}
			u.RawQuery = c.xPaddingKey + "=" + paddingValue
			req.Header.Set(c.xPaddingHeader, u.String())
		case PlacementCookie:
			req.AddCookie(&http.Cookie{Name: c.xPaddingKey, Value: paddingValue, Path: "/"})
		case PlacementQuery:
			q := req.URL.Query()
			q.Set(c.xPaddingKey, paddingValue)
			req.URL.RawQuery = q.Encode()
		}
	} else {
		// Default Xray behavior: place padding as x_padding query in Referer header
		u, err := url.Parse(rawURL)
		if err != nil {
			return
		}
		u.RawQuery = "x_padding=" + paddingValue
		req.Header.Set("Referer", u.String())
	}
}

// applyXPaddingToResponseHeader applies padding to response headers
func (c *xhttpConfig) applyXPaddingToResponseHeader(h http.Header) {
	length := randRange(c.xPaddingBytesFrom, c.xPaddingBytesTo)
	if length <= 0 {
		return
	}
	paddingValue := generatePadding(c.xPaddingMethod, int(length))

	if c.xPaddingObfsMode {
		switch c.xPaddingPlacement {
		case PlacementHeader:
			h.Set(c.xPaddingHeader, paddingValue)
		case PlacementQueryInHeader:
			h.Set(c.xPaddingHeader, "?"+c.xPaddingKey+"="+paddingValue)
		default:
			h.Set("X-Padding", paddingValue)
		}
	} else {
		h.Set("X-Padding", paddingValue)
	}
}

// extractXPaddingFromRequest extracts padding value from a request for validation
func (c *xhttpConfig) extractXPaddingFromRequest(req *http.Request) string {
	if !c.xPaddingObfsMode {
		// Default: check Referer header for x_padding query
		referrer := req.Header.Get("Referer")
		if referrer != "" {
			if referrerURL, err := url.Parse(referrer); err == nil {
				return referrerURL.Query().Get("x_padding")
			}
		}
		// Fallback: check URL query
		return req.URL.Query().Get("x_padding")
	}

	// Obfs mode
	key := c.xPaddingKey
	header := c.xPaddingHeader

	if cookie, err := req.Cookie(key); err == nil && cookie.Value != "" {
		return cookie.Value
	}
	headerValue := req.Header.Get(header)
	if headerValue != "" {
		if c.xPaddingPlacement == PlacementHeader {
			return headerValue
		}
		if parsedURL, err := url.Parse(headerValue); err == nil {
			return parsedURL.Query().Get(key)
		}
	}
	return req.URL.Query().Get(key)
}

// validatePadding checks if a padding value is valid
func (c *xhttpConfig) validatePadding(paddingValue string) bool {
	if paddingValue == "" {
		return true // Accept missing padding for compatibility
	}
	from := c.xPaddingBytesFrom
	to := c.xPaddingBytesTo

	switch c.xPaddingMethod {
	case PaddingMethodTokenish:
		n := int32(hpack.HuffmanEncodeLength(paddingValue))
		return n >= from-2 && n <= to+2
	default:
		n := int32(len(paddingValue))
		return n >= from && n <= to
	}
}

// isUplinkRequest checks if this is an uplink request based on method and data placement
func (c *xhttpConfig) isUplinkRequest(req *http.Request) bool {
	if c.uplinkHTTPMethod != "GET" && req.Method == c.uplinkHTTPMethod {
		return true
	}
	switch c.uplinkDataPlacement {
	case PlacementHeader:
		if req.Header.Get(c.uplinkDataKey+"-Upstream") == "1" {
			return true
		}
	case PlacementCookie:
		if cookie, _ := req.Cookie(c.uplinkDataKey + "_upstream"); cookie != nil && cookie.Value == "1" {
			return true
		}
	}
	return false
}

// encodeUplinkData encodes data for non-body placement and applies to request
func (c *xhttpConfig) encodeUplinkData(req *http.Request, data []byte) {
	encodedData := base64.RawURLEncoding.EncodeToString(data)
	key := c.uplinkDataKey
	chunkSize := c.uplinkChunkSize

	switch c.uplinkDataPlacement {
	case PlacementHeader:
		for i := 0; i < len(encodedData); i += chunkSize {
			end := i + chunkSize
			if end > len(encodedData) {
				end = len(encodedData)
			}
			headerKey := fmt.Sprintf("%s-%d", key, i/chunkSize)
			req.Header.Set(headerKey, encodedData[i:end])
		}
		req.Header.Set(key+"-Length", fmt.Sprintf("%d", len(encodedData)))
		req.Header.Set(key+"-Upstream", "1")
	case PlacementCookie:
		for i := 0; i < len(encodedData); i += chunkSize {
			end := i + chunkSize
			if end > len(encodedData) {
				end = len(encodedData)
			}
			cookieName := fmt.Sprintf("%s_%d", key, i/chunkSize)
			req.AddCookie(&http.Cookie{Name: cookieName, Value: encodedData[i:end]})
		}
		req.AddCookie(&http.Cookie{Name: key + "_upstream", Value: "1"})
	}
}

// decodeUplinkData extracts data from non-body placement in a request
func (c *xhttpConfig) decodeUplinkData(req *http.Request) ([]byte, error) {
	key := c.uplinkDataKey
	var encodedStr string

	switch c.uplinkDataPlacement {
	case PlacementHeader:
		dataLenStr := req.Header.Get(key + "-Length")
		if dataLenStr != "" {
			dataLen, _ := strconv.Atoi(dataLenStr)
			var chunks []string
			for i := 0; ; i++ {
				chunk := req.Header.Get(fmt.Sprintf("%s-%d", key, i))
				if chunk == "" {
					break
				}
				chunks = append(chunks, chunk)
			}
			encodedStr = strings.Join(chunks, "")
			if len(encodedStr) != dataLen {
				encodedStr = ""
			}
		}
	case PlacementCookie:
		var chunks []string
		for i := 0; ; i++ {
			cookieName := fmt.Sprintf("%s_%d", key, i)
			if cookie, _ := req.Cookie(cookieName); cookie != nil {
				chunks = append(chunks, cookie.Value)
			} else {
				break
			}
		}
		if len(chunks) > 0 {
			encodedStr = strings.Join(chunks, "")
		}
	}

	if encodedStr == "" {
		return nil, fmt.Errorf("no data found in %s placement with key %s", c.uplinkDataPlacement, key)
	}
	return base64.RawURLEncoding.DecodeString(encodedStr)
}

// generatePadding creates a padding string of the given method and length
func generatePadding(method string, length int) string {
	if length <= 0 {
		return ""
	}
	switch method {
	case PaddingMethodTokenish:
		return generateTokenishPadding(length)
	default: // repeat-x
		return strings.Repeat("X", length)
	}
}

// generateTokenishPadding generates a base62 string that has approximately
// the target length when Huffman-encoded
func generateTokenishPadding(targetHuffmanBytes int) string {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const avgHuffmanBytesPerChar = 0.8
	n := int(float64(targetHuffmanBytes)/avgHuffmanBytesPerChar + 0.5)
	if n < 1 {
		n = 1
	}

	result := make([]byte, n)
	buf := make([]byte, 256)
	i := 0
	limit := byte(256 - (256 % len(charset)))
	for i < n {
		rand.Read(buf)
		for _, rb := range buf {
			if rb >= limit {
				continue
			}
			result[i] = charset[int(rb)%len(charset)]
			i++
			if i >= n {
				break
			}
		}
	}

	s := string(result)
	// Adjust to hit target
	for iter := 0; iter < 150; iter++ {
		currentLen := int(hpack.HuffmanEncodeLength(s))
		diff := currentLen - targetHuffmanBytes
		if diff >= -2 && diff <= 2 {
			return s
		}
		if diff < 0 {
			s += "X"
		} else if len(s) > 1 {
			s = s[:len(s)-1]
		} else {
			return s
		}
	}
	return s
}

// randRange returns a random number in [from, to]
func randRange(from, to int32) int32 {
	if from >= to {
		return from
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(to-from+1)))
	if err != nil {
		return from
	}
	return from + int32(n.Int64())
}

func appendToPath(path, value string) string {
	if strings.HasSuffix(path, "/") {
		return path + value
	}
	return path + "/" + value
}
