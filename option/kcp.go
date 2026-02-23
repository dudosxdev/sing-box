package option

// V2RayKCPOptions contains KCP transport options.
type V2RayKCPOptions struct {
	// MTU is the maximum transmission unit in bytes. Default: 1350.
	MTU uint32 `json:"mtu,omitempty"`
	// TTI is the transmission time interval in milliseconds. Default: 50.
	TTI uint32 `json:"tti,omitempty"`
	// UplinkCapacity is the uplink capacity in MB/s. Default: 5.
	UplinkCapacity uint32 `json:"uplink_capacity,omitempty"`
	// DownlinkCapacity is the downlink capacity in MB/s. Default: 20.
	DownlinkCapacity uint32 `json:"downlink_capacity,omitempty"`
	// Congestion enables congestion control. Default: false.
	Congestion bool `json:"congestion,omitempty"`
	// WriteBufferSize is the write buffer size in bytes. Default: 2MB.
	WriteBufferSize uint32 `json:"write_buffer_size,omitempty"`
	// ReadBufferSize is the read buffer size in bytes. Default: 2MB.
	ReadBufferSize uint32 `json:"read_buffer_size,omitempty"`
	// Seed is the obfuscation seed.
	Seed string `json:"seed,omitempty"`
	// Header is the packet header type for obfuscation.
	Header *V2RayKCPHeaderOptions `json:"header,omitempty"`
}

// V2RayKCPHeaderOptions contains KCP header obfuscation options.
type V2RayKCPHeaderOptions struct {
	// Type is the header type: none, srtp, utp, wechat-video, dtls, wireguard, dns.
	Type string `json:"type,omitempty"`
	// Domain is used for dns header type.
	Domain string `json:"domain,omitempty"`
}

// V2RayMKCPOptions contains mKCP (finalmask) transport options.
type V2RayMKCPOptions struct {
	// MTU is the maximum transmission unit in bytes. Default: 1350.
	MTU uint32 `json:"mtu,omitempty"`
	// TTI is the transmission time interval in milliseconds. Default: 50.
	TTI uint32 `json:"tti,omitempty"`
	// UplinkCapacity is the uplink capacity in MB/s. Default: 5.
	UplinkCapacity uint32 `json:"uplink_capacity,omitempty"`
	// DownlinkCapacity is the downlink capacity in MB/s. Default: 20.
	DownlinkCapacity uint32 `json:"downlink_capacity,omitempty"`
	// Congestion enables congestion control. Default: false.
	Congestion bool `json:"congestion,omitempty"`
	// WriteBufferSize is the write buffer size in bytes. Default: 2MB.
	WriteBufferSize uint32 `json:"write_buffer_size,omitempty"`
	// ReadBufferSize is the read buffer size in bytes. Default: 2MB.
	ReadBufferSize uint32 `json:"read_buffer_size,omitempty"`
	// Seed is the obfuscation seed.
	Seed string `json:"seed,omitempty"`
	// Masks is the list of UDP masks to apply.
	Masks []V2RayMKCPMaskOptions `json:"masks,omitempty"`
}

// V2RayMKCPMaskOptions contains a single mask configuration.
type V2RayMKCPMaskOptions struct {
	// Type is the mask type: original, aes128gcm, srtp, utp, wechat-video, dtls, wireguard, dns, salamander.
	Type string `json:"type,omitempty"`
	// Password is used for aes128gcm and salamander mask types.
	Password string `json:"password,omitempty"`
	// Domain is used for dns mask type.
	Domain string `json:"domain,omitempty"`
}
