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
