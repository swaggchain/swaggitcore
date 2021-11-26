package network

import (
	"errors"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	libnet "github.com/libp2p/go-libp2p-core/network"
	bloom "github.com/bits-and-blooms/bloom/v3"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
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
	id peer.ID
	addr multiaddr.Multiaddr
	conn libnet.Conn
	netManager *NetworkManager
	stream libnet.Stream
	continuousTimeout int
	recentMsg *bloom.BloomFilter
	bloomMutex sync.Mutex
	bloomItemCount int
	urgentMsgCh chan *p2pMessage
	normalMsgCh chan *p2pMessage
	direction connDirection
	quitWriteCh chan struct{}
	once sync.Once
	lastRoutingQueryTime atomic.Int64
}

func CreateNewPeer(stream libnet.Stream, nm *NetworkManager, direction connDirection) *Peer {
	p := &Peer{
		id: stream.Conn().RemotePeer(),
		addr: stream.Conn().RemoteMultiaddr(),
		conn: stream.Conn(),
		stream: stream,
		netManager: nm,
		recentMsg: bloom.NewWithEstimates(bloomMaxItemCount, bloomErrRate),
		urgentMsgCh: make(chan *p2pMessage, msgChanSize),
		normalMsgCh: make(chan *p2pMessage, msgChanSize),
		quitWriteCh: make(chan struct{}),
		direction: direction,
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

	deadline := time.Now().Add(time.Duration(len(m.raw())/1024/5+3))
	if err := p.stream.SetWriteDeadline(deadline); err != nil {
		log.Warnf("setting write deadline failed. err=%v, pid=%v", err, p.ID())
		return err
	}

	return nil

}

