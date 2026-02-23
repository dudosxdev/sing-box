package v2rayxhttp

import (
	"container/heap"
	"io"
	"runtime"
	"sync"
)

// Packet represents a data chunk with a sequence number
type Packet struct {
	Reader  io.ReadCloser
	Payload []byte
	Seq     uint64
}

// uploadQueue is a specialized priority queue + channel to reorder
// generic packets by a sequence number
type uploadQueue struct {
	reader          io.ReadCloser
	nomore          bool
	pushedPackets   chan Packet
	writeCloseMutex sync.Mutex
	heap            uploadHeap
	nextSeq         uint64
	closed          bool
	maxPackets      int
}

func newUploadQueue(maxPackets int) *uploadQueue {
	return &uploadQueue{
		pushedPackets: make(chan Packet, maxPackets),
		heap:          uploadHeap{},
		nextSeq:       0,
		closed:        false,
		maxPackets:    maxPackets,
	}
}

func (h *uploadQueue) Push(p Packet) error {
	h.writeCloseMutex.Lock()
	defer h.writeCloseMutex.Unlock()

	if h.closed {
		return io.ErrClosedPipe
	}
	if h.nomore {
		return io.ErrClosedPipe
	}
	if p.Reader != nil {
		h.nomore = true
	}
	h.pushedPackets <- p
	return nil
}

func (h *uploadQueue) Close() error {
	h.writeCloseMutex.Lock()
	defer h.writeCloseMutex.Unlock()

	if !h.closed {
		h.closed = true
		runtime.Gosched()
	f:
		for {
			select {
			case p := <-h.pushedPackets:
				if p.Reader != nil {
					h.reader = p.Reader
				}
			default:
				break f
			}
		}
		close(h.pushedPackets)
	}
	if h.reader != nil {
		return h.reader.Close()
	}
	return nil
}

func (h *uploadQueue) Read(b []byte) (int, error) {
	if h.reader != nil {
		return h.reader.Read(b)
	}

	if h.closed {
		return 0, io.EOF
	}

	if len(h.heap) == 0 {
		packet, more := <-h.pushedPackets
		if !more {
			return 0, io.EOF
		}
		if packet.Reader != nil {
			h.reader = packet.Reader
			return h.reader.Read(b)
		}
		heap.Push(&h.heap, packet)
	}

	for len(h.heap) > 0 {
		packet := heap.Pop(&h.heap).(Packet)

		if packet.Seq == h.nextSeq {
			n := copy(b, packet.Payload)

			if n < len(packet.Payload) {
				packet.Payload = packet.Payload[n:]
				heap.Push(&h.heap, packet)
			} else {
				h.nextSeq = packet.Seq + 1
			}

			return n, nil
		}

		// misordered packet
		if packet.Seq > h.nextSeq {
			if len(h.heap) > h.maxPackets {
				return 0, io.ErrUnexpectedEOF
			}
			heap.Push(&h.heap, packet)
			packet2, more := <-h.pushedPackets
			if !more {
				return 0, io.EOF
			}
			heap.Push(&h.heap, packet2)
		}
	}

	return 0, nil
}

// uploadHeap implements heap.Interface for Packet ordering
type uploadHeap []Packet

func (h uploadHeap) Len() int           { return len(h) }
func (h uploadHeap) Less(i, j int) bool { return h[i].Seq < h[j].Seq }
func (h uploadHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *uploadHeap) Push(x any) {
	*h = append(*h, x.(Packet))
}

func (h *uploadHeap) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
