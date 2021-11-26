package network

import (
	"encoding/binary"
	"errors"
	"github.com/golang/snappy"
	"github.com/libp2p/go-libp2p-core/peer"
	"hash/crc32"
	"strconv"
)

/*

	Message protocol as follow:

	POSITIONS = LAST ONE IS NOT INCLUDED (ie.: 0, 4 ... 0,1,2,3 = where the bytes are)

	CHAIN_ID (4 byte)  Position: 0, 4   uint32
	MSG_TYPE (2 byte) Position: 4, 6	uint16
	VERSION (2 byte) Position: 6, 8	uint16
	DATA_SIZE (4 byte) Position: 8, 12   uint32
	IS_COMPRESSED (1 byte) Position: 12 bool
	IS_ENCRYPTED (1  byte) Position: 13 bool
	RESERVED_SPECIAL_ROUTING (2 byte) Position: 14, 16 uint16
	DATA_START (max 1000 byte) Position: 16 []byte
	CHKSUM (SIZE - 10) uint32


 */

type p2pMessage []byte

type MessageType uint16


const (
	_ MessageType = iota
	RoutingTableQuery
	RoutingTableResponse
	NewBlock
	NewBlockHash
	NewBlockRequest
	SyncBlockHashRequest
	SyncBlockHashResponse
	SyncBlockRequest
	SyncBlockResponse
	SyncHeight
	PublishTx

	UrgentMessage = 1
	NormalMessage = 2
)

type MessagePriority int

func (m MessageType) String() string {
	switch m {
	case RoutingTableQuery:
		return "RoutingTableQuery"
	case RoutingTableResponse:
		return "RoutingTableResponse"
	case NewBlock:
		return "NewBlock"
	case NewBlockRequest:
		return "NewBlockRequest"
	case SyncBlockHashRequest:
		return "SyncBlockHashRequest"
	case SyncBlockHashResponse:
		return "SyncBlockHashResponse"
	case SyncBlockRequest:
		return "SyncBlockRequest"
	case SyncBlockResponse:
		return "SyncBlockResponse"
	case SyncHeight:
		return "SyncHeight"
	case PublishTx:
		return "PublishTx"
	case NewBlockHash:
		return "NewBlockHash"
	default:
		return "unknown_type:" + strconv.Itoa(int(m))
	}
}


type MessagePackets struct {
	chainID uint32
	msgType MessageType
	version uint16
	size uint32
	checksum uint32
	isCompressed uint16
	isEncrypted uint16
	specialRouting uint16
	data []byte
}

func (p *p2pMessage) Encode(m *MessagePackets) *p2pMessage {

	//check compression
	if m.isCompressed > 0 {
		m.data = snappy.Encode(nil, m.data)
	}
	content := make([]byte, 18 + len(m.data))
	binary.BigEndian.PutUint32(content, m.chainID)
	binary.BigEndian.PutUint16(content[4:6], uint16(m.msgType))
	binary.BigEndian.PutUint16(content[6:8], m.version)
	binary.BigEndian.PutUint32(content[8:12], m.size)
	binary.BigEndian.PutUint16(content[12:14], m.isCompressed)
	binary.BigEndian.PutUint16(content[14:16], m.isEncrypted)
	binary.BigEndian.PutUint16(content[16:18], m.specialRouting)
	binary.BigEndian.PutUint32(content[18:22], crc32.ChecksumIEEE(m.data))
	copy(content[22:], m.data)

	pm := p2pMessage(content)
	return &pm

}

func (p *p2pMessage) raw() []byte {
	return []byte(*p)
}

func (p *p2pMessage) chainID() uint32 {
	return binary.BigEndian.Uint32(p.raw()[0:4])
}

func (p *p2pMessage) msgType() MessageType {
	return MessageType(binary.BigEndian.Uint16(p.raw()[4:6]))
}

func (p *p2pMessage) version() uint16 {
	return binary.BigEndian.Uint16(p.raw()[6:8])
}

func (p *p2pMessage) size() uint32 {
	return binary.BigEndian.Uint32(p.raw()[8:12])
}

func (p *p2pMessage) isCompressed() uint16 {
	return binary.BigEndian.Uint16(p.raw()[12:14])
}

func (p *p2pMessage) isEncrypted() uint16 {
	return binary.BigEndian.Uint16(p.raw()[14:16])
}

func (p *p2pMessage) specialRouting() uint16 {
	return binary.BigEndian.Uint16(p.raw()[16:18])
}

func (p *p2pMessage) rawData() []byte {
	return p.raw()[22:]
}

func (p *p2pMessage) checksum() uint32 {
	return binary.BigEndian.Uint32(p.raw()[18:22])
}

func (p *p2pMessage) needDedup() bool {
	return p.msgType() == PublishTx || p.msgType() == NewBlockHash
}

func DecodeRawMessage(rawBytes []byte) (*MessagePackets, error) {

	p := p2pMessage(rawBytes)

	m := &MessagePackets{
		chainID:        p.chainID(),
		msgType:        p.msgType(),
		version:        p.version(),
		size:           p.size(),
		checksum:       p.checksum(),
		isCompressed:   p.isCompressed(),
		isEncrypted:    p.isEncrypted(),
		specialRouting: p.specialRouting(),
		data:           p.rawData(),
	}

	if p.isCompressed() > 0 {
		m.data, _ = snappy.Decode(nil, m.data)
	}

	if len(rawBytes) < 22 {
		return nil, errors.New("message is too short: size mismatch")
	}

	if len(m.data) != int(m.size) {
		return nil, errors.New("size mismatch")
	}

	if m.checksum != crc32.ChecksumIEEE(m.data) {
		return nil, errors.New("invalid checksum")
	}

	return m, nil


}

type IncomingMessage struct {
	from peer.ID
	data []byte
	typ MessageType
}

func NewIncomingMessage(peerID peer.ID, data []byte, messageType MessageType) *IncomingMessage {
	return &IncomingMessage{peerID, data, messageType}
}

func (m *IncomingMessage) From() peer.ID {
	return m.from
}
func (m *IncomingMessage) Data() []byte {
	return m.data
}

func (m *IncomingMessage) Type() MessageType {
	return m.typ
}

func newP2PMessage(chainID uint32, typ MessageType, cflag uint16, data []byte) *p2pMessage {
	p2pm := new(p2pMessage)
	msg := p2pm.Encode(&MessagePackets{
		chainID:chainID,
		msgType: typ,
		isCompressed: cflag,
		data: data,
	})
	return msg
}
