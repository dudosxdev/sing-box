package option

import "github.com/sagernet/sing/common/json/badoption"

// V2RayXHTTPRangeConfig represents a range with from/to values
type V2RayXHTTPRangeConfig struct {
	From int32 `json:"from,omitempty"`
	To   int32 `json:"to,omitempty"`
}

// V2RayXHTTPXmuxConfig represents xmux (connection multiplexing) settings
type V2RayXHTTPXmuxConfig struct {
	MaxConcurrency  *V2RayXHTTPRangeConfig `json:"max_concurrency,omitempty"`
	MaxConnections  *V2RayXHTTPRangeConfig `json:"max_connections,omitempty"`
	CMaxReuseTimes  *V2RayXHTTPRangeConfig `json:"c_max_reuse_times,omitempty"`
	HMaxRequestTimes *V2RayXHTTPRangeConfig `json:"h_max_request_times,omitempty"`
	HMaxReusableSecs *V2RayXHTTPRangeConfig `json:"h_max_reusable_secs,omitempty"`
	HKeepAlivePeriod int64                  `json:"h_keep_alive_period,omitempty"`
}

type V2RayXHTTPOptions struct {
	Host    string               `json:"host,omitempty"`
	Path    string               `json:"path,omitempty"`
	Mode    string               `json:"mode,omitempty"`
	Headers badoption.HTTPHeader `json:"headers,omitempty"`

	// Padding settings
	XPaddingBytes     *V2RayXHTTPRangeConfig `json:"x_padding_bytes,omitempty"`
	XPaddingObfsMode  bool                   `json:"x_padding_obfs_mode,omitempty"`
	XPaddingKey       string                 `json:"x_padding_key,omitempty"`
	XPaddingHeader    string                 `json:"x_padding_header,omitempty"`
	XPaddingPlacement string                 `json:"x_padding_placement,omitempty"`
	XPaddingMethod    string                 `json:"x_padding_method,omitempty"`

	// Response headers
	NoGRPCHeader bool `json:"no_grpc_header,omitempty"`
	NoSSEHeader  bool `json:"no_sse_header,omitempty"`

	// Upload settings
	UplinkHTTPMethod    string `json:"uplink_http_method,omitempty"`
	UplinkDataPlacement string `json:"uplink_data_placement,omitempty"`
	UplinkDataKey       string `json:"uplink_data_key,omitempty"`
	UplinkChunkSize     uint32 `json:"uplink_chunk_size,omitempty"`

	// Session/seq placement
	SessionPlacement string `json:"session_placement,omitempty"`
	SessionKey       string `json:"session_key,omitempty"`
	SeqPlacement     string `json:"seq_placement,omitempty"`
	SeqKey           string `json:"seq_key,omitempty"`

	// Packet-up mode settings
	ScMaxEachPostBytes   *V2RayXHTTPRangeConfig `json:"sc_max_each_post_bytes,omitempty"`
	ScMinPostsIntervalMs *V2RayXHTTPRangeConfig `json:"sc_min_posts_interval_ms,omitempty"`
	ScMaxBufferedPosts   int                    `json:"sc_max_buffered_posts,omitempty"`

	// Stream-up server response interval
	ScStreamUpServerSecs *V2RayXHTTPRangeConfig `json:"sc_stream_up_server_secs,omitempty"`

	// Xmux settings (client-side connection multiplexing)
	Xmux *V2RayXHTTPXmuxConfig `json:"xmux,omitempty"`
}
