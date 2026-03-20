V2Ray Transport 是 v2ray 发明的一组私有协议，并污染了其他协议的名称，如 clash 中的 `trojan-grpc`。

### 结构

```json
{
  "type": ""
}
```

可用的传输协议：

* HTTP
* WebSocket
* QUIC
* gRPC
* HTTPUpgrade
* xHTTP (splitHTTP)

!!! warning "与 v2ray-core 的区别"

    * 没有 TCP 传输层, 纯 HTTP 已合并到 HTTP 传输层。
    * 没有 mKCP 传输层。
    * 没有 DomainSocket 传输层。

!!! note ""

    当内容只有一项时，可以忽略 JSON 数组 [] 标签。

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

!!! warning "与 v2ray-core 的区别"

    不强制执行 TLS。如果未配置 TLS，将使用纯 HTTP 1.1。

#### host

主机域名列表。

如果设置，客户端将随机选择，服务器将验证。

#### path

!!! warning

    V2Ray 文档称服务端和客户端的路径必须一致，但实际代码允许客户端向路径添加任何后缀。
    sing-box 使用与 V2Ray 相同的行为，但请注意，该行为在 `WebSocket` 和 `HTTPUpgrade` 传输层中不存在。

HTTP 请求路径

服务器将验证。

#### method

HTTP 请求方法

如果设置，服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

#### idle_timeout

在 HTTP2 服务器中：

指定闲置客户端应在多长时间内使用 GOAWAY 帧关闭。PING 帧不被视为活动。

在 HTTP2 客户端中：

如果连接上没有收到任何帧，指定一段时间后将使用 PING 帧执行健康检查。需要注意的是，PING 响应被视为已接收的帧，因此如果连接上没有其他流量，则健康检查将在每个间隔执行一次。如果值为零，则不会执行健康检查。

默认使用零。

#### ping_timeout

在 HTTP2 客户端中：

指定发送 PING 帧后，在指定的超时时间内必须接收到响应。如果在指定的超时时间内没有收到 PING 帧的响应，则连接将关闭。默认超时持续时间为 15 秒。

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

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头

如果设置，服务器将写入响应。

#### max_early_data

请求中允许的最大有效负载大小。默认启用。

#### early_data_header_name

默认情况下，早期数据在路径而不是标头中发送。

要与 Xray-core 兼容，请将其设置为 `Sec-WebSocket-Protocol`。

它需要与服务器保持一致。

### QUIC

```json
{
  "type": "quic"
}
```

!!! warning "与 v2ray-core 的区别"

    没有额外的加密支持：
    它基本上是重复加密。 并且 Xray-core 在这里与 v2ray-core 不兼容。

### gRPC

!!! note ""

    默认安装不包含标准 gRPC (兼容性好，但性能较差), 参阅 [安装](/zh/installation/build-from-source/#构建标记)。

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

gRPC 服务名称。

#### idle_timeout

在标准 gRPC 服务器/客户端：

如果传输在此时间段后没有看到任何活动，它会向客户端发送 ping 请求以检查连接是否仍然活动。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### ping_timeout

在标准 gRPC 服务器/客户端：

经过一段时间之后，客户端将执行 keepalive 检查并等待活动。如果没有检测到任何活动，则会关闭连接。

在默认 gRPC 服务器/客户端：

它的行为与 HTTP 传输层中的相应设置相同。

#### permit_without_stream

在标准 gRPC 客户端：

如果启用，客户端传输即使没有活动连接也会发送 keepalive ping。如果禁用，则在没有活动连接时，将忽略 `idle_timeout` 和 `ping_timeout`，并且不会发送 keepalive ping。

默认禁用。

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

主机域名。

服务器将验证。

#### path

HTTP 请求路径

服务器将验证。

#### headers

HTTP 请求的额外标头。

如果设置，服务器将写入响应。

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

!!! info "兼容性"

    完全兼容 Xray-core xHTTP/splitHTTP 传输协议。已通过 Xray-core v26+ 验证。
    支持 `"xhttp"` 和 `"splithttp"` 两种类型名称作为别名。

!!! note "传输模式"

    xHTTP 支持三种工作模式：

    * **`packet-up`**（默认）— 通过单独的 POST 请求上传（每个带有序列号），通过长连接 GET 下载（SSE 流式）。与 CDN 和反向代理的兼容性最好。
    * **`stream-up`** — 通过单个长连接 POST 上传，通过单个长连接 GET 下载。开销更低，但需要 HTTP/2 或 h2c 以获得最佳效果。
    * **`stream-one`** — 单个双向 HTTP 请求（上传请求体 + 流式响应）。最简单的模式，但 CDN 兼容性有限。

#### host

主机域名。

如果设置，服务器将验证。

#### path

HTTP 请求路径。可以在 `?` 后附加查询字符串。

路径会自动规范化：如果缺少前导 `/` 会自动添加，并在末尾追加 `/`。

服务器将验证路径前缀。

#### mode

传输模式。可选 `packet-up`、`stream-up`、`stream-one` 或空（默认为 `packet-up`）。

如果显式设置，客户端和服务器必须一致。

#### headers

HTTP 请求的额外标头。

如果未指定，会添加默认的 `User-Agent` 标头（类 Chrome）。

#### x_padding_bytes

范围 `{from, to}`，每个请求/响应添加的随机填充大小（字节）。

默认：`{from: 100, to: 1000}`。

#### x_padding_obfs_mode

如果为 `true`，填充使用自定义混淆放置方式（参见 `x_padding_placement`、`x_padding_key`、`x_padding_header`）。

如果为 `false`（默认），填充作为 `Referer` 标头中的 `x_padding` 查询参数放置（标准 Xray 行为）。

#### x_padding_key

混淆模式下填充值的键名（用于 query/cookie 放置）。

#### x_padding_header

混淆模式下填充值的标头名（用于 header/queryInHeader 放置）。

#### x_padding_placement

混淆模式下填充的放置位置：

| 值 | 描述 |
|---|------|
| `header` | 填充作为自定义标头值放置 |
| `queryInHeader` | 填充作为标头中 URL 的查询参数放置 |
| `cookie` | 填充作为 Cookie 放置 |
| `query` | 填充作为 URL 查询参数放置 |

#### x_padding_method

填充生成方法：

| 值 | 描述 |
|---|------|
| `repeat-x`（默认） | 重复的 `X` 字符 |
| `tokenish` | 按 Huffman 编码长度调整大小的 Base62 随机字符串 |

#### no_grpc_header

如果为 `true`，上传请求不添加 `Content-Type: application/grpc` 标头。

默认：`false`。

#### no_sse_header

如果为 `true`，下载响应不添加 `Content-Type: text/event-stream` 标头。

默认：`false`。

#### uplink_http_method

上传请求的 HTTP 方法。

默认：`POST`。

#### uplink_data_placement

上传数据的放置位置：

| 值 | 描述 |
|---|------|
| `body`（默认） | 数据在请求体中 |
| `header` | 数据 Base64 编码后分块放入标头 |
| `cookie` | 数据 Base64 编码后分块放入 Cookie |

使用 `header` 或 `cookie` 时，需要设置 `uplink_data_key`。

#### uplink_data_key

非请求体数据放置的键前缀。`header` 模式下，分块命名为 `{key}-0`、`{key}-1` 等，总长度放在 `{key}-Length`。`cookie` 模式下，分块命名为 `{key}_0`、`{key}_1` 等。

#### uplink_chunk_size

非请求体放置时每个编码数据块的最大大小。

默认：`1000000`。

#### session_placement

会话 ID 的放置位置：

| 值 | 描述 |
|---|------|
| `path`（默认） | 附加到 URL 路径 |
| `query` | 作为 URL 查询参数 |
| `header` | 作为请求标头 |
| `cookie` | 作为 Cookie |

#### session_key

非 `path` 放置时会话 ID 的键名。

默认值：`header` 模式为 `X-Session`，`query`/`cookie` 模式为 `x_session`。

#### seq_placement

序列号的放置位置（仅 packet-up 模式）：

| 值 | 描述 |
|---|------|
| `path`（默认） | 附加到 URL 路径 |
| `query` | 作为 URL 查询参数 |
| `header` | 作为请求标头 |
| `cookie` | 作为 Cookie |

#### seq_key

非 `path` 放置时序列号的键名。

默认值：`header` 模式为 `X-Seq`，`query`/`cookie` 模式为 `x_seq`。

#### sc_max_each_post_bytes

范围 `{from, to}`，上传 POST 请求体的最大大小（字节，packet-up 模式）。

默认：`{from: 1000000, to: 1000000}`。

#### sc_min_posts_interval_ms

范围 `{from, to}`，连续 POST 请求之间的最小间隔（毫秒，packet-up 模式）。

默认：`{from: 30, to: 30}`。

#### sc_max_buffered_posts

重排序前缓冲的最大乱序数据包数量（packet-up 模式）。

默认：`30`。

#### sc_stream_up_server_secs

范围 `{from, to}`，服务器保活响应间隔（秒，stream-up 模式）。

默认：`{from: 20, to: 80}`。

#### xmux

客户端连接复用设置。仅在客户端生效。

##### max_concurrency

范围 `{from, to}`。每个 HTTP/2 连接的最大并发流数。

##### max_connections

范围 `{from, to}`。要维护的 HTTP/2 连接目标数量。

##### c_max_reuse_times

范围 `{from, to}`。连接可被重用于新流的最大次数。

##### h_max_request_times

范围 `{from, to}`。每个连接的最大 HTTP 请求次数。

##### h_max_reusable_secs

范围 `{from, to}`。连接可被重用的最大时间（秒）。

##### h_keep_alive_period

保活周期（秒）。尚未实现。

#### 完整示例：sing-box ↔ sing-box

**sing-box 服务端：**
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

**sing-box 客户端：**
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

#### 完整示例：sing-box 客户端 → Xray-core 服务端

**Xray-core 服务端：**
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

**sing-box 客户端：**
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

#### 完整示例：Xray-core 客户端 → sing-box 服务端

**sing-box 服务端：**
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

**Xray-core 客户端：**
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

#### Xray-core 配置映射

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
