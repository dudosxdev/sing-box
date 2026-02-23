package kcp

// Config holds KCP configuration parameters.
type Config struct {
	MTU              uint32
	TTI              uint32
	UplinkCapacity   uint32
	DownlinkCapacity uint32
	Congestion       bool
	WriteBufferSize  uint32
	ReadBufferSize   uint32
	Seed             string
}

func (c *Config) GetMTUValue() uint32 {
	if c == nil || c.MTU == 0 {
		return 1350
	}
	return c.MTU
}

func (c *Config) GetTTIValue() uint32 {
	if c == nil || c.TTI == 0 {
		return 50
	}
	return c.TTI
}

func (c *Config) GetUplinkCapacityValue() uint32 {
	if c == nil || c.UplinkCapacity == 0 {
		return 5
	}
	return c.UplinkCapacity
}

func (c *Config) GetDownlinkCapacityValue() uint32 {
	if c == nil || c.DownlinkCapacity == 0 {
		return 20
	}
	return c.DownlinkCapacity
}

func (c *Config) GetWriteBufferSize() uint32 {
	if c == nil || c.WriteBufferSize == 0 {
		return 2 * 1024 * 1024
	}
	return c.WriteBufferSize
}

func (c *Config) GetSendingInFlightSize() uint32 {
	size := c.GetUplinkCapacityValue() * 1024 * 1024 / c.GetMTUValue() / (1000 / c.GetTTIValue())
	if size < 8 {
		size = 8
	}
	return size
}

func (c *Config) GetSendingBufferSize() uint32 {
	return c.GetWriteBufferSize() / c.GetMTUValue()
}

func (c *Config) GetReceivingInFlightSize() uint32 {
	size := c.GetDownlinkCapacityValue() * 1024 * 1024 / c.GetMTUValue() / (1000 / c.GetTTIValue())
	if size < 8 {
		size = 8
	}
	return size
}
