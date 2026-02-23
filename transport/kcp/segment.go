// Package kcp - A Fast and Reliable ARQ Protocol
// Ported from Xray-core/transport/internet/kcp
package kcp

import (
	"encoding/binary"

	"github.com/sagernet/sing/common/buf"
)

// Command is a KCP command that indicate the purpose of a Segment.
type Command byte

const (
	CommandACK       Command = 0
	CommandData      Command = 1
	CommandTerminate Command = 2
	CommandPing      Command = 3
)

type SegmentOption byte

const (
	SegmentOptionClose SegmentOption = 1
)

type Segment interface {
	Release()
	Conversation() uint16
	Command() Command
	ByteSize() int32
	Serialize([]byte)
	parse(conv uint16, cmd Command, opt SegmentOption, b []byte) (bool, []byte)
}

const (
	DataSegmentOverhead = 18
)

type DataSegment struct {
	Conv        uint16
	Option      SegmentOption
	Timestamp   uint32
	Number      uint32
	SendingNext uint32

	payload  *buf.Buffer
	timeout  uint32
	transmit uint32
}

func NewDataSegment() *DataSegment {
	return new(DataSegment)
}

func (s *DataSegment) parse(conv uint16, cmd Command, opt SegmentOption, b []byte) (bool, []byte) {
	s.Conv = conv
	s.Option = opt
	if len(b) < 15 {
		return false, nil
	}
	s.Timestamp = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.Number = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.SendingNext = binary.BigEndian.Uint32(b)
	b = b[4:]
	dataLen := int(binary.BigEndian.Uint16(b))
	b = b[2:]
	if len(b) < dataLen {
		return false, nil
	}
	s.Data().Reset()
	s.Data().Write(b[:dataLen])
	b = b[dataLen:]
	return true, b
}

func (s *DataSegment) Conversation() uint16 { return s.Conv }
func (*DataSegment) Command() Command        { return CommandData }

func (s *DataSegment) Detach() *buf.Buffer {
	r := s.payload
	s.payload = nil
	return r
}

func (s *DataSegment) Data() *buf.Buffer {
	if s.payload == nil {
		s.payload = buf.New()
	}
	return s.payload
}

func (s *DataSegment) Serialize(b []byte) {
	binary.BigEndian.PutUint16(b, s.Conv)
	b[2] = byte(CommandData)
	b[3] = byte(s.Option)
	binary.BigEndian.PutUint32(b[4:], s.Timestamp)
	binary.BigEndian.PutUint32(b[8:], s.Number)
	binary.BigEndian.PutUint32(b[12:], s.SendingNext)
	binary.BigEndian.PutUint16(b[16:], uint16(s.payload.Len()))
	copy(b[18:], s.payload.Bytes())
}

func (s *DataSegment) ByteSize() int32 {
	return int32(2 + 1 + 1 + 4 + 4 + 4 + 2 + s.payload.Len())
}

func (s *DataSegment) Release() {
	s.payload.Release()
	s.payload = nil
}

type AckSegment struct {
	Conv            uint16
	Option          SegmentOption
	ReceivingWindow uint32
	ReceivingNext   uint32
	Timestamp       uint32
	NumberList      []uint32
}

const ackNumberLimit = 128

func NewAckSegment() *AckSegment { return new(AckSegment) }

func (s *AckSegment) parse(conv uint16, cmd Command, opt SegmentOption, b []byte) (bool, []byte) {
	s.Conv = conv
	s.Option = opt
	if len(b) < 13 {
		return false, nil
	}
	s.ReceivingWindow = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.ReceivingNext = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.Timestamp = binary.BigEndian.Uint32(b)
	b = b[4:]
	count := int(b[0])
	b = b[1:]
	if len(b) < count*4 {
		return false, nil
	}
	for i := 0; i < count; i++ {
		s.PutNumber(binary.BigEndian.Uint32(b))
		b = b[4:]
	}
	return true, b
}

func (s *AckSegment) Conversation() uint16 { return s.Conv }
func (*AckSegment) Command() Command        { return CommandACK }

func (s *AckSegment) PutTimestamp(timestamp uint32) {
	if timestamp-s.Timestamp < 0x7FFFFFFF {
		s.Timestamp = timestamp
	}
}

func (s *AckSegment) PutNumber(number uint32) {
	s.NumberList = append(s.NumberList, number)
}

func (s *AckSegment) IsFull() bool  { return len(s.NumberList) == ackNumberLimit }
func (s *AckSegment) IsEmpty() bool { return len(s.NumberList) == 0 }

func (s *AckSegment) ByteSize() int32 {
	return 2 + 1 + 1 + 4 + 4 + 4 + 1 + int32(len(s.NumberList)*4)
}

func (s *AckSegment) Serialize(b []byte) {
	binary.BigEndian.PutUint16(b, s.Conv)
	b[2] = byte(CommandACK)
	b[3] = byte(s.Option)
	binary.BigEndian.PutUint32(b[4:], s.ReceivingWindow)
	binary.BigEndian.PutUint32(b[8:], s.ReceivingNext)
	binary.BigEndian.PutUint32(b[12:], s.Timestamp)
	b[16] = byte(len(s.NumberList))
	n := 17
	for _, number := range s.NumberList {
		binary.BigEndian.PutUint32(b[n:], number)
		n += 4
	}
}

func (s *AckSegment) Release() {}

type CmdOnlySegment struct {
	Conv          uint16
	Cmd           Command
	Option        SegmentOption
	SendingNext   uint32
	ReceivingNext uint32
	PeerRTO       uint32
}

func NewCmdOnlySegment() *CmdOnlySegment { return new(CmdOnlySegment) }

func (s *CmdOnlySegment) parse(conv uint16, cmd Command, opt SegmentOption, b []byte) (bool, []byte) {
	s.Conv = conv
	s.Cmd = cmd
	s.Option = opt
	if len(b) < 12 {
		return false, nil
	}
	s.SendingNext = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.ReceivingNext = binary.BigEndian.Uint32(b)
	b = b[4:]
	s.PeerRTO = binary.BigEndian.Uint32(b)
	b = b[4:]
	return true, b
}

func (s *CmdOnlySegment) Conversation() uint16 { return s.Conv }
func (s *CmdOnlySegment) Command() Command      { return s.Cmd }
func (*CmdOnlySegment) ByteSize() int32         { return 2 + 1 + 1 + 4 + 4 + 4 }

func (s *CmdOnlySegment) Serialize(b []byte) {
	binary.BigEndian.PutUint16(b, s.Conv)
	b[2] = byte(s.Cmd)
	b[3] = byte(s.Option)
	binary.BigEndian.PutUint32(b[4:], s.SendingNext)
	binary.BigEndian.PutUint32(b[8:], s.ReceivingNext)
	binary.BigEndian.PutUint32(b[12:], s.PeerRTO)
}

func (*CmdOnlySegment) Release() {}

func ReadSegment(b []byte) (Segment, []byte) {
	if len(b) < 4 {
		return nil, nil
	}
	conv := binary.BigEndian.Uint16(b)
	b = b[2:]
	cmd := Command(b[0])
	opt := SegmentOption(b[1])
	b = b[2:]

	var seg Segment
	switch cmd {
	case CommandData:
		seg = NewDataSegment()
	case CommandACK:
		seg = NewAckSegment()
	default:
		seg = NewCmdOnlySegment()
	}

	valid, extra := seg.parse(conv, cmd, opt, b)
	if !valid {
		return nil, nil
	}
	return seg, extra
}
