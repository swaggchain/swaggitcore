package network

import (
	"encoding/binary"
	"errors"
	bloom "github.com/bits-and-blooms/bloom/v3"
	libnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"io"
	"strings"
	"sync"
	"time"
)

var (
	ErrMessageChannelFull = errors.New("message channel is full")
	ErrDuplicateMessage   = errors.New("reduplicate message")
)

const (
	bloomMaxItemCount = 100000
	bloomErrRate      = 0.001

	msgChanSize          = 1024
	maxDataLength        = 10000000 // 10MB
	routingQueryTimeout  = 10
	maxContinuousTimeout = 10
)

type connDirection int

type Peer struct {
	id                   peer.ID
	addr                 multiaddr.Multiaddr
	conn                 libnet.Conn
	netManager           *NetworkManager
	stream               libnet.Stream
	continuousTimeout    int
	recentMsg            *bloom.BloomFilter
	bloomMutex           sync.Mutex
	bloomItemCount       int
	urgentMsgCh          chan *p2pMessage
	normalMsgCh          chan *p2pMessage
	direction            connDirection
	quitWriteCh          chan struct{}
	once                 sync.Once
	lastRoutingQueryTime atomic.Int64
}

func NewPeer(stream libnet.Stream, nm *NetworkManager, direction connDirection) *Peer {
	p := &Peer{
		id:          stream.Conn().RemotePeer(),
		addr:        stream.Conn().RemoteMultiaddr(),
		conn:        stream.Conn(),
		stream:      stream,
		netManager:  nm,
		recentMsg:   bloom.NewWithEstimates(bloomMaxItemCount, bloomErrRate),
		urgentMsgCh: make(chan *p2pMessage, msgChanSize),
		normalMsgCh: make(chan *p2pMessage, msgChanSize),
		quitWriteCh: make(chan struct{}),
		direction:   direction,
	}
	p.lastRoutingQueryTime.Store(time.Now().Unix())
	return p
}

func (p *Peer) ID() string {
	return p.id.Pretty()
}

func (p *Peer) Addr() string {
	return p.addr.String()
}

func (p *Peer) Start() {
	log.Infof("peer is started. id=%s", p.ID())

	go p.readLoop()
	go p.writeLoop()

}

func (p *Peer) Stop() {
	log.Infof("peer is stopped. id=%s", p.ID())
	p.once.Do(func() {
		close(p.quitWriteCh)
	})
	p.conn.Close()
}

func (p *Peer) write(m *p2pMessage) error {

	deadline := time.Now().Add(time.Duration(len(m.raw())/1024/5 + 3))
	if err := p.stream.SetWriteDeadline(deadline); err != nil {
		log.Warnf("setting write deadline failed. err=%v, pid=%v", err, p.ID())
		return err
	}

	_, err := p.stream.Write(m.raw())
	if err != nil {
		log.Warnf("writing message failed. err=%v, pid=%v", err, p.ID())
		if strings.Contains(err.Error(), "i/o timeout") {
			p.continuousTimeout++
			if p.continuousTimeout >= maxContinuousTimeout {
				log.Warnf("max continuous timeout times, remove peer %v", p.ID())
				p.netManager.RemoveNeighbor(p.id)
			}
			return err
		}

	}

	p.continuousTimeout = 0
	//tagkv := map[string]string{"mtype": m.msgType().String()}
	//byteOutCounter.Add(float64(len(m.raw())),tagkv)
	//packetOutCounter.Add(1, tagkv)

	return nil

}

func (p *Peer) writeLoop() {

	for {
		select {
		case <-p.quitWriteCh:
			log.Infof("peer is stopped. pid=%v, addr=%v", p.ID(), p.addr)
			return
		case um := <-p.urgentMsgCh:
			_ = p.write(um)

		case nm := <-p.normalMsgCh:
			for done := false; !done; {
				select {
				case <-p.quitWriteCh:
					return
				case um := <-p.urgentMsgCh:
					_ = p.write(um)
				default:
					done = true
				}
			}
			_ = p.write(nm)
		}
	}

}

func (p *Peer) readLoop() {

	header := make([]byte, 22)
	for {
		_, err := io.ReadFull(p.stream, header)
		if err != nil {
			log.Warnf("read header failed. err=%v", err)
			break
		}
		chainID := binary.BigEndian.Uint32(header[0:4])
		if chainID != p.netManager.config.ChainID {
			log.Warnf("Mismatched chainID, put peer to blacklist. remotePeer=%v, chainID=%d", p.ID(), chainID)
			p.netManager.PutPeerToBlack(p.ID())
			return
		}
		length := binary.BigEndian.Uint32(header[8:12])
		if length > maxDataLength {
			log.Warnf("data length too large: %d", length)
			break
		}
		data := make([]byte, 22+length)
		_, err = io.ReadFull(p.stream, data[22:])
		if err != nil {
			log.Warnf("read message failed. err=%v", err)
			break
		}
		copy(data[0:22], header)
		msg := p2pMessage(data)
		if err != nil {
			log.Errorf("parse p2pmessage failed. err=%v", err)
			break
		}
		//	tagkv := map[string]string{"mtype": msg.msgType().String()}
		//byteInCounter.Add(float64(len(msg.data)), tagkv)
		//packetInCounter.Add(1, tagkv)
		p.handleMessage(&msg)
	}
	p.netManager.RemoveNeighbor(p.id)
}

func (p *Peer) SendMessage(msg *p2pMessage, mp MessagePriority, deduplicate bool) error {

	if deduplicate && msg.needDedup() {
		if p.hasMessage(msg) {
			return errors.New("duplicate message")
		}
	}

	ch := p.urgentMsgCh
	if mp == NormalMessage {
		ch = p.normalMsgCh
	}

	select {
	case ch <- msg:
	default:
		log.Errorf("sending message failed. channel is full. messagePriority=%d", mp)
		return ErrMessageChannelFull
	}

	if msg.needDedup() {
		p.recordMessage(msg)
	}
	if msg.msgType() == RoutingTableQuery {
		p.routingQueryNow()
	}

	return nil
}

func (p *Peer) handleMessage(msg *p2pMessage) error {
	if msg.needDedup() {
		p.recordMessage(msg)
	}
	if msg.msgType() == RoutingTableResponse {
		if p.isRoutingQueryTimeout() {
			log.Debugf("receive timeout routing response. pid=%v", p.ID())
			return nil
		}
		p.resetRoutingQueryTime()
	}
	p.netManager.HandleMessage(msg, p.id)
	return nil
}

func (p *Peer) recordMessage(msg *p2pMessage) {
	p.bloomMutex.Lock()
	defer p.bloomMutex.Unlock()

	if p.bloomItemCount >= bloomMaxItemCount {
		p.recentMsg = bloom.NewWithEstimates(bloomMaxItemCount, bloomErrRate)
		p.bloomItemCount = 0
	}

	p.recentMsg.Add(msg.rawData())
	p.bloomItemCount++
}

func (p *Peer) hasMessage(msg *p2pMessage) bool {
	p.bloomMutex.Lock()
	defer p.bloomMutex.Unlock()

	return p.recentMsg.Test(msg.rawData())
}

func (p *Peer) resetRoutingQueryTime() {
	p.lastRoutingQueryTime.Store(-1)
}

func (p *Peer) isRoutingQueryTimeout() bool {
	return time.Now().Unix()-p.lastRoutingQueryTime.Load() > routingQueryTimeout
}

func (p *Peer) routingQueryNow() {
	p.lastRoutingQueryTime.Store(time.Now().Unix())
}
