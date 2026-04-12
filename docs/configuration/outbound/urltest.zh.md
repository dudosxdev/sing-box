### 结构

```json
{
  "type": "urltest",
  "tag": "auto",
  
  "outbounds": [
    "proxy-a",
    "proxy-b",
    "proxy-c"
  ],
  "url": "",
  "interval": "",
  "unavailable_check_interval": "",
  "tolerance": 50,
  "idle_timeout": "",
  "interrupt_exist_connections": false
}
```

### 字段

#### outbounds

==必填==

用于测试的出站标签列表。

#### url

用于测试的链接。默认使用 `https://www.gstatic.com/generate_204`。

#### interval

测试间隔。 默认使用 `3m`。

#### unavailable_check_interval

某个出站因真实流量失败而被标记为不可用后，执行一次单独重测前的等待时间。默认使用 `2s`。

只会对这个已经不可用的出站重测一次。如果它恢复，可用状态和延迟会立即写回 URLTest 历史，并且组选择会立刻重新计算，而不需要等待下一次常规 `interval`。

#### tolerance

以毫秒为单位的测试容差。 默认使用 `50`。

#### idle_timeout

空闲超时。默认使用 `30m`。

#### interrupt_exist_connections

当选定的出站发生更改时，中断现有连接。

仅入站连接受此设置影响，内部连接将始终被中断。
