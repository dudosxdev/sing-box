V2Ray Transport is a set of private protocols invented by v2ray, and has contaminated the names of other protocols, such
as `trojan-grpc` in clash.

### Structure

```json
{
  "type": ""
}
```

Available transports:

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* mKCP (finalmask)
* xHTTP (splitHTTP)

!!! warning "Difference from v2ray-core"

    * No TCP transport, plain HTTP is merged into the HTTP transport.
    * No DomainSocket transport.

!!! note ""

    You can ignore the JSON Array [] tag when the content is only one item

### HTTP

```json
{
  "type": "http",
  "host": [],
  "path": "",
  "method": "",
  "headers": {},
  "idle_timeout": "15s",
  "ping_timeout": "15s"
}
```

!!! warning "Difference from v2ray-core"

    TLS is not enforced. If TLS is not configured, plain HTTP 1.1 is used.

#### host

List of host domain.

The client will choose randomly and the server will verify if not empty.

#### path

!!! warning

    V2Ray's documentation says that the path between the server and the client must be consistent, 
    but the actual code allows the client to add any suffix to the path.
    sing-box uses the same behavior as V2Ray, but note that the behavior does not exist in `WebSocket` and `HTTPUpgrade` transport.

Path of HTTP request.

The server will verify.

#### method

Method of HTTP request.

The server will verify if not empty.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### idle_timeout

In HTTP2 server:

Specifies the time until idle clients should be closed with a GOAWAY frame. PING frames are not considered as activity.

In HTTP2 client:

Specifies the period of time after which a health check will be performed using a ping frame if no frames have been
received on the connection.Please note that a ping response is considered a received frame, so if there is no other
traffic on the connection, the health check will be executed every interval. If the value is zero, no health check will
be performed.

Zero is used by default.

#### ping_timeout

In HTTP2 client:

Specifies the timeout duration after sending a PING frame, within which a response must be received.
If a response to the PING frame is not received within the specified timeout duration, the connection will be closed.
The default timeout duration is 15 seconds.

### WebSocket

```json
{
  "type": "ws",
  "path": "",
  "headers": {},
  "max_early_data": 0,
  "early_data_header_name": ""
}
```

#### path

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

#### max_early_data

Allowed payload size is in the request. Enabled if not zero.

#### early_data_header_name

Early data is sent in path instead of header by default.

To be compatible with Xray-core, set this to `Sec-WebSocket-Protocol`.

It needs to be consistent with the server.

### QUIC

```json
{
  "type": "quic"
}
```

!!! warning "Difference from v2ray-core"

    No additional encryption support:
    It's basically duplicate encryption. And Xray-core is not compatible with v2ray-core in here.

### gRPC

!!! note ""

    standard gRPC has good compatibility but poor performance and is not included by default, see [Installation](/installation/build-from-source/#build-tags).

```json
{
  "type": "grpc",
  "service_name": "TunService",
  "idle_timeout": "15s",
  "ping_timeout": "15s",
  "permit_without_stream": false
}
```

#### service_name

Service name of gRPC.

#### idle_timeout

In standard gRPC server/client:

If the transport doesn't see any activity after a duration of this time,
it pings the client to check if the connection is still active.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### ping_timeout

In standard gRPC server/client:

The timeout that after performing a keepalive check, the client will wait for activity.
If no activity is detected, the connection will be closed.

In default gRPC server/client:

It has the same behavior as the corresponding setting in HTTP transport.

#### permit_without_stream

In standard gRPC client:

If enabled, the client transport sends keepalive pings even with no active connections.
If disabled, when there are no active connections, `idle_timeout` and `ping_timeout` will be ignored and no keepalive
pings will be sent.

Disabled by default.

### HTTPUpgrade

```json
{
  "type": "httpupgrade",
  "host": "",
  "path": "",
  "headers": {}
}
```

#### host

Host domain.

The server will verify if not empty.

#### path

Path of HTTP request.

The server will verify.

#### headers

Extra headers of HTTP request.

The server will write in response if not empty.

### mKCP (finalmask)

!!! note ""

    mKCP transport requires the build tag `with_v2ray_transport_mkcp`.

```json
{
  "type": "mkcp",
  "mtu": 1350,
  "tti": 50,
  "uplink_capacity": 5,
  "downlink_capacity": 20,
  "congestion": false,
  "write_buffer_size": 2097152,
  "read_buffer_size": 2097152,
  "masks": [
    {
      "type": "original"
    }
  ]
}
```

!!! info "Compatibility"

    Compatible with Xray-core v26+ finalmask system. Works as both client and server.
    The masks configuration on client and server must match exactly.

#### mtu

Maximum Transmission Unit in bytes.

Default: `1350`.

#### tti

Transmission Time Interval in milliseconds. Lower values reduce latency but increase bandwidth usage.

Default: `50`.

#### uplink_capacity

Uplink bandwidth capacity in MB/s. Used by the congestion control algorithm.

Default: `5`.

#### downlink_capacity

Downlink bandwidth capacity in MB/s. Used by the congestion control algorithm.

Default: `20`.

#### congestion

Enable built-in congestion control.

Default: `false`.

#### write_buffer_size

Per-connection write buffer size in bytes.

Default: `2097152` (2 MB).

#### read_buffer_size

Per-connection read buffer size in bytes.

Default: `2097152` (2 MB).

#### masks

List of UDP masks to apply. Masks are chained in order — the first mask is the outermost (applied last on send, first on receive), the last mask manages the raw buffer.

Each mask object has a `type` field and optional parameters:

##### Mask types

| Type | Description | Extra fields |
|------|-------------|-------------|
| `original` | FNV32a XOR authentication (6-byte overhead) | — |
| `aes128gcm` | AES-128-GCM encryption (28-byte overhead) | `password` |
| `srtp` | SRTP packet header (4 bytes) | — |
| `utp` | µTP (µTorrent) packet header (4 bytes) | — |
| `wechat-video` | WeChat Video Call packet header (13 bytes) | — |
| `dtls` | DTLS 1.2 packet header (13 bytes) | — |
| `wireguard` | WireGuard packet header (4 bytes) | — |
| `dns` | DNS query packet header (variable) | `domain` |

##### Mask examples

**Basic authentication (original):**
```json
{
  "masks": [{"type": "original"}]
}
```

**AES-128-GCM encryption:**
```json
{
  "masks": [{"type": "aes128gcm", "password": "my-secret"}]
}
```

**Authentication + SRTP header disguise:**
```json
{
  "masks": [{"type": "original"}, {"type": "srtp"}]
}
```

**AES encryption + WireGuard header disguise:**
```json
{
  "masks": [{"type": "aes128gcm", "password": "my-secret"}, {"type": "wireguard"}]
}
```

**Authentication + DNS header disguise:**
```json
{
  "masks": [{"type": "original"}, {"type": "dns", "domain": "www.example.com"}]
}
```

#### Xray-core config mapping

When using with an Xray-core v26+ server/client, the mask types map as follows:

| sing-box | Xray-core finalmask |
|----------|-------------------|
| `original` | `mkcp-original` |
| `aes128gcm` | `mkcp-aes128gcm` |
| `srtp` | `header-srtp` |
| `utp` | `header-utp` |
| `wechat-video` | `header-wechat` |
| `dtls` | `header-dtls` |
| `wireguard` | `header-wireguard` |
| `dns` | `header-dns` |

!!! warning "Xray-core config format"

    In Xray-core, mask parameters like `password` and `domain` must be placed inside a `"settings"` object:
    ```json
    {
      "finalmask": {
        "udp": [
          {"type": "mkcp-aes128gcm", "settings": {"password": "my-secret"}},
          {"type": "header-srtp"}
        ]
      }
    }
    ```

#### Full example: sing-box client → Xray server

**sing-box client:**
```json
{
  "outbounds": [{
    "type": "vmess",
    "server": "example.com",
    "server_port": 443,
    "uuid": "your-uuid",
    "security": "auto",
    "transport": {
      "type": "mkcp",
      "masks": [
        {"type": "aes128gcm", "password": "my-secret"},
        {"type": "srtp"}
      ]
    }
  }]
}
```

**Xray-core v26 server:**
```json
{
  "inbounds": [{
    "port": 443,
    "protocol": "vmess",
    "settings": {"clients": [{"id": "your-uuid"}]},
    "streamSettings": {
      "network": "kcp",
      "finalmask": {
        "udp": [
          {"type": "mkcp-aes128gcm", "settings": {"password": "my-secret"}},
          {"type": "header-srtp"}
        ]
      }
    }
  }]
}
```

#### Full example: sing-box server + sing-box client

**sing-box server:**
```json
{
  "inbounds": [{
    "type": "vmess",
    "listen": "0.0.0.0",
    "listen_port": 443,
    "users": [{"uuid": "your-uuid"}],
    "transport": {
      "type": "mkcp",
      "masks": [
        {"type": "original"},
        {"type": "wireguard"}
      ]
    }
  }],
  "outbounds": [{"type": "direct"}]
}
```

**sing-box client:**
```json
{
  "outbounds": [{
    "type": "vmess",
    "server": "example.com",
    "server_port": 443,
    "uuid": "your-uuid",
    "security": "auto",
    "transport": {
      "type": "mkcp",
      "masks": [
        {"type": "original"},
        {"type": "wireguard"}
      ]
    }
  }]
}
```

### xHTTP (splitHTTP)

```json
{
  "type": "xhttp",
  "host": "",
  "path": "/",
  "mode": "",
  "headers": {},

  "x_padding_bytes": {
    "from": 100,
    "to": 1000
  },
  "x_padding_obfs_mode": false,
  "x_padding_key": "",
  "x_padding_header": "",
  "x_padding_placement": "",
  "x_padding_method": "",

  "no_grpc_header": false,
  "no_sse_header": false,

  "uplink_http_method": "POST",
  "uplink_data_placement": "",
  "uplink_data_key": "",
  "uplink_chunk_size": 1000000,

  "session_placement": "",
  "session_key": "",
  "seq_placement": "",
  "seq_key": "",

  "sc_max_each_post_bytes": {
    "from": 1000000,
    "to": 1000000
  },
  "sc_min_posts_interval_ms": {
    "from": 30,
    "to": 30
  },
  "sc_max_buffered_posts": 30,
  "sc_stream_up_server_secs": {
    "from": 20,
    "to": 80
  },

  "xmux": {
    "max_concurrency": {
      "from": 0,
      "to": 0
    },
    "max_connections": {
      "from": 0,
      "to": 0
    },
    "c_max_reuse_times": {
      "from": 0,
      "to": 0
    },
    "h_max_request_times": {
      "from": 0,
      "to": 0
    },
    "h_max_reusable_secs": {
      "from": 0,
      "to": 0
    },
    "h_keep_alive_period": 0
  }
}
```

!!! info "Compatibility"

    Fully compatible with Xray-core xHTTP/splitHTTP transport. Verified with Xray-core v26+.
    Both `"xhttp"` and `"splithttp"` type names are accepted as aliases.

!!! note "Transport modes"

    xHTTP supports three operation modes:

    * **`packet-up`** (default) — Upload via individual POST requests (each with a sequence number), download via long-lived GET (SSE-style stream). Best compatibility with CDNs and reverse proxies.
    * **`stream-up`** — Upload via a single long-lived POST, download via a single long-lived GET. Lower overhead but requires HTTP/2 or h2c for best results.
    * **`stream-one`** — Single bidirectional HTTP request (upload body + streaming response). Simplest mode but limited CDN compatibility.

#### host

Host domain.

The server will verify if not empty.

#### path

Path of HTTP request. A query string can be appended after `?`.

The path is automatically normalized: a leading `/` is added if missing, and a trailing `/` is appended.

The server will verify the path prefix.

#### mode

Transport mode. One of `packet-up`, `stream-up`, `stream-one`, or empty (defaults to `packet-up`).

Must be consistent between client and server if set explicitly.

#### headers

Extra headers of HTTP request.

A default `User-Agent` header (Chrome-like) is added if not specified.

#### x_padding_bytes

Range `{from, to}` for the random padding size in bytes added to each request/response.

Default: `{from: 100, to: 1000}`.

#### x_padding_obfs_mode

If `true`, padding is placed using custom obfuscation placement (see `x_padding_placement`, `x_padding_key`, `x_padding_header`).

If `false` (default), padding is placed as `x_padding` query parameter in the `Referer` header (standard Xray behavior).

#### x_padding_key

Key name for padding value in obfs mode (used in query/cookie placement).

#### x_padding_header

Header name for padding value in obfs mode (used in header/queryInHeader placement).

#### x_padding_placement

Where to place padding in obfs mode. One of:

| Value | Description |
|-------|-------------|
| `header` | Padding placed as a custom header value |
| `queryInHeader` | Padding placed as a query in a URL stored in a header |
| `cookie` | Padding placed as a cookie |
| `query` | Padding placed as a URL query parameter |

#### x_padding_method

Padding generation method:

| Value | Description |
|-------|-------------|
| `repeat-x` (default) | Repeated `X` characters |
| `tokenish` | Base62 random string sized by Huffman-encoded length |

#### no_grpc_header

If `true`, the `Content-Type: application/grpc` header is not added to upload requests.

Default: `false`.

#### no_sse_header

If `true`, the `Content-Type: text/event-stream` header is not added to download responses.

Default: `false`.

#### uplink_http_method

HTTP method for upload requests.

Default: `POST`.

#### uplink_data_placement

Where to place upload data:

| Value | Description |
|-------|-------------|
| `body` (default) | Data in the request body |
| `header` | Data base64-encoded in chunked headers |
| `cookie` | Data base64-encoded in chunked cookies |

When using `header` or `cookie`, `uplink_data_key` is required.

#### uplink_data_key

Key prefix for non-body data placement. For `header` mode, chunks are named `{key}-0`, `{key}-1`, etc. with `{key}-Length` for total length. For `cookie` mode, chunks are named `{key}_0`, `{key}_1`, etc.

#### uplink_chunk_size

Maximum size of each encoded data chunk for non-body placement.

Default: `1000000`.

#### session_placement

Where to place the session ID:

| Value | Description |
|-------|-------------|
| `path` (default) | Appended to the URL path |
| `query` | As a URL query parameter |
| `header` | As a request header |
| `cookie` | As a cookie |

#### session_key

Key name for the session ID when not using `path` placement.

Defaults: `X-Session` for header, `x_session` for query/cookie.

#### seq_placement

Where to place the sequence number (packet-up mode only):

| Value | Description |
|-------|-------------|
| `path` (default) | Appended to the URL path |
| `query` | As a URL query parameter |
| `header` | As a request header |
| `cookie` | As a cookie |

#### seq_key

Key name for the sequence number when not using `path` placement.

Defaults: `X-Seq` for header, `x_seq` for query/cookie.

#### sc_max_each_post_bytes

Range `{from, to}` for maximum upload POST body size in bytes (packet-up mode).

Default: `{from: 1000000, to: 1000000}`.

#### sc_min_posts_interval_ms

Range `{from, to}` for minimum interval between consecutive POST requests in milliseconds (packet-up mode).

Default: `{from: 30, to: 30}`.

#### sc_max_buffered_posts

Maximum number of out-of-order packets to buffer before reordering (packet-up mode).

Default: `30`.

#### sc_stream_up_server_secs

Range `{from, to}` for the server keep-alive response interval in seconds (stream-up mode).

Default: `{from: 20, to: 80}`.

#### xmux

Client-side connection multiplexing settings. Only effective on the client.

##### max_concurrency

Range `{from, to}`. Maximum number of concurrent streams per HTTP/2 connection.

##### max_connections

Range `{from, to}`. Target number of HTTP/2 connections to maintain.

##### c_max_reuse_times

Range `{from, to}`. Maximum number of times a connection can be reused for new streams.

##### h_max_request_times

Range `{from, to}`. Maximum number of HTTP requests per connection.

##### h_max_reusable_secs

Range `{from, to}`. Maximum time in seconds a connection can be reused.

##### h_keep_alive_period

Keep-alive period in seconds. Not yet implemented.

#### Full example: sing-box ↔ sing-box

**sing-box server:**
```json
{
  "inbounds": [{
    "type": "vless",
    "listen": "0.0.0.0",
    "listen_port": 443,
    "users": [{"uuid": "your-uuid"}],
    "transport": {
      "type": "xhttp",
      "path": "/secret-path"
    }
  }],
  "outbounds": [{"type": "direct"}]
}
```

**sing-box client:**
```json
{
  "outbounds": [{
    "type": "vless",
    "server": "example.com",
    "server_port": 443,
    "uuid": "your-uuid",
    "transport": {
      "type": "xhttp",
      "path": "/secret-path"
    }
  }]
}
```

#### Full example: sing-box client → Xray-core server

**Xray-core server:**
```json
{
  "inbounds": [{
    "listen": "0.0.0.0",
    "port": 443,
    "protocol": "vless",
    "settings": {
      "clients": [{"id": "your-uuid"}],
      "decryption": "none"
    },
    "streamSettings": {
      "network": "xhttp",
      "xhttpSettings": {
        "path": "/secret-path"
      }
    }
  }],
  "outbounds": [{"protocol": "freedom"}]
}
```

**sing-box client:**
```json
{
  "outbounds": [{
    "type": "vless",
    "server": "example.com",
    "server_port": 443,
    "uuid": "your-uuid",
    "transport": {
      "type": "xhttp",
      "path": "/secret-path"
    }
  }]
}
```

#### Full example: Xray-core client → sing-box server

**sing-box server:**
```json
{
  "inbounds": [{
    "type": "vless",
    "listen": "0.0.0.0",
    "listen_port": 443,
    "users": [{"uuid": "your-uuid"}],
    "transport": {
      "type": "xhttp",
      "path": "/secret-path"
    }
  }],
  "outbounds": [{"type": "direct"}]
}
```

**Xray-core client:**
```json
{
  "outbounds": [{
    "protocol": "vless",
    "settings": {
      "vnext": [{
        "address": "example.com",
        "port": 443,
        "users": [{"id": "your-uuid", "encryption": "none"}]
      }]
    },
    "streamSettings": {
      "network": "xhttp",
      "xhttpSettings": {
        "path": "/secret-path"
      }
    }
  }]
}
```

#### Advanced example: obfuscated padding with custom placement

```json
{
  "type": "xhttp",
  "path": "/cdn-path",
  "mode": "packet-up",
  "x_padding_bytes": {"from": 50, "to": 200},
  "x_padding_obfs_mode": true,
  "x_padding_method": "tokenish",
  "x_padding_placement": "cookie",
  "x_padding_key": "cf_session"
}
```

#### Advanced example: non-body data placement

```json
{
  "type": "xhttp",
  "path": "/api/v1/data",
  "uplink_http_method": "GET",
  "uplink_data_placement": "header",
  "uplink_data_key": "X-Data",
  "session_placement": "header",
  "session_key": "X-Request-Id",
  "seq_placement": "query",
  "seq_key": "page"
}
```

#### Advanced example: xmux connection pooling (client only)

```json
{
  "type": "xhttp",
  "path": "/mux-path",
  "xmux": {
    "max_concurrency": {"from": 8, "to": 16},
    "max_connections": {"from": 2, "to": 4},
    "c_max_reuse_times": {"from": 50, "to": 100},
    "h_max_request_times": {"from": 200, "to": 400},
    "h_max_reusable_secs": {"from": 120, "to": 300}
  }
}
```

#### Xray-core config mapping

| sing-box | Xray-core (xhttpSettings) |
|----------|---------------------------|
| `host` | `host` |
| `path` | `path` |
| `mode` | `mode` |
| `headers` | `headers` |
| `x_padding_bytes` | `xPaddingBytes` |
| `x_padding_obfs_mode` | `xPaddingObfsMode` |
| `x_padding_key` | `xPaddingKey` |
| `x_padding_header` | `xPaddingHeader` |
| `x_padding_placement` | `xPaddingPlacement` |
| `x_padding_method` | `xPaddingMethod` |
| `no_grpc_header` | `noGRPCHeader` |
| `no_sse_header` | `noSSEHeader` |
| `uplink_http_method` | `uplinkHTTPMethod` |
| `uplink_data_placement` | `uplinkDataPlacement` |
| `uplink_data_key` | `uplinkDataKey` |
| `uplink_chunk_size` | `uplinkChunkSize` |
| `session_placement` | `sessionPlacement` |
| `session_key` | `sessionKey` |
| `seq_placement` | `seqPlacement` |
| `seq_key` | `seqKey` |
| `sc_max_each_post_bytes` | `scMaxEachPostBytes` |
| `sc_min_posts_interval_ms` | `scMinPostsIntervalMs` |
| `sc_max_buffered_posts` | `scMaxBufferedPosts` |
| `sc_stream_up_server_secs` | `scStreamUpServerSecs` |
| `xmux.max_concurrency` | `xmux.maxConcurrency` |
| `xmux.max_connections` | `xmux.maxConnections` |
| `xmux.c_max_reuse_times` | `xmux.cMaxReuseTimes` |
| `xmux.h_max_request_times` | `xmux.hMaxRequestTimes` |
| `xmux.h_max_reusable_secs` | `xmux.hMaxReusableSecs` |
| `xmux.h_keep_alive_period` | `xmux.hKeepAlivePeriod` |
