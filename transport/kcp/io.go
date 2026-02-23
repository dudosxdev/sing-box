package kcp

type PacketReader interface {
	Read([]byte) []Segment
}

// KCPPacketReader reads KCP segments from raw bytes.
// This matches Xray-core's KCPPacketReader which does NOT use SimpleAuthenticator.
type KCPPacketReader struct{}

func (r *KCPPacketReader) Read(b []byte) []Segment {
	var result []Segment
	for len(b) > 0 {
		seg, x := ReadSegment(b)
		if seg == nil {
			break
		}
		result = append(result, seg)
		b = x
	}
	return result
}
