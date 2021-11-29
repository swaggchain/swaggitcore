package network

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/libp2p/go-libp2p-core/host"
	libnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/libp2p/go-libp2p-core/peerstore"
	manet "github.com/multiformats/go-multiaddr/net"
	log "github.com/sirupsen/logrus"
	"go.uber.org/atomic"
	"math/rand"
	"os"
	"path/filepath"
	"strings"

	kbucket "github.com/libp2p/go-libp2p-kbucket"
	multiaddr "github.com/multiformats/go-multiaddr"
	madns "github.com/multiformats/go-multiaddr-dns"
	netpb "swagg/services/network/pb"
	_ "swagg/services/types"
	"sync"
	"time"
)

var (
	dumpRoutingTableInterval = 5 * time.Minute
	syncRoutingTableInterval = 30 * time.Second
	metricsStatInterval      = 3 * time.Second
	findBPInterval           = 2 * time.Second

	dialTimeout        = 10 * time.Second
	deadPeerRetryTimes = 5
)

const (
	inbound connDirection = iota
	outbound
)

const (
	defaultOutboundConn = 10
	defaultInboundConn  = 20

	bucketSize        = 1000
	peerResponseCount = 20
	maxPeerQuery      = 30
	maxAddrCount      = 10

	incomingMsgChanSize = 4096

	routingTableFile = "routing.table"
)

type NetworkManager struct {
	neighbors     map[peer.ID]*Peer
	neighborCount map[connDirection]int
	neighborMutex sync.RWMutex

	neighborCap map[connDirection]int

	subs    *sync.Map
	quitCh  chan struct{}
	started atomic.Int32

	host             host.Host
	config           *NetConfig
	routingTable     *kbucket.RoutingTable
	peerStore        peerstore.Peerstore
	lastUpdateTime   atomic.Int64
	wg               *sync.WaitGroup
	blockProducers   []peer.ID
	blockProducersMu sync.RWMutex
	validators       []peer.ID
	validatorsMu     sync.RWMutex
	blackListPID     map[string]bool
	blackListIP      map[string]bool
	blackListMu      sync.RWMutex
	retryTimes       map[string]int
	rtMutex          sync.RWMutex
}

type NetConfig struct {
	ListenAddr   string
	SeedNodes    []string
	ChainID      uint32
	Version      uint16
	DataPath     string
	InboundConn  int
	OutboundConn int
	BlackIP      []string
	BlackPID     []string
	AdminPort    string
}

func NewNetworkManager(host host.Host, config *NetConfig) *NetworkManager {

	routingTable, _ := kbucket.NewRoutingTable(1000, kbucket.ConvertPeerID(host.ID()), time.Second, host.Peerstore(), time.Minute, nil)

	nm := &NetworkManager{
		neighbors:     make(map[peer.ID]*Peer),
		neighborCount: make(map[connDirection]int),
		neighborCap:   make(map[connDirection]int),
		subs:          new(sync.Map),
		quitCh:        make(chan struct{}),
		routingTable:  routingTable,
		host:          host,
		config:        config,
		peerStore:     host.Peerstore(),
		wg:            new(sync.WaitGroup),
		blackListPID:  make(map[string]bool),
		blackListIP:   make(map[string]bool),
		retryTimes:    make(map[string]int),
	}

	return nm

}

func (nm *NetworkManager) Start() {

	if !nm.started.CAS(0, 1) {
		return
	}

	nm.parseSeeds()
	nm.LoadRoutingTable()
	nm.wg.Add(4)
	go nm.dumpRoutingTableLoop()
	go nm.syncRoutingTableLoop()
	go nm.metricsStatLoop()
	go nm.findBlockProducerLoop()

}

func (nm *NetworkManager) Stop() {

	if !nm.started.CAS(1, 0) {
		return
	}

	close(nm.quitCh)
	nm.wg.Wait()
	nm.CloseAllNeighbors()

}

func (nm *NetworkManager) isStopped() bool {
	return nm.started.Load() == 0
}

func (nm *NetworkManager) setBlockProducers(ids []string) {
	peerIDs := make([]peer.ID, 0, len(ids))

	for _, id := range ids {
		if len(id) == 0 {
			continue
		}
		peerID, err := peer.Decode(id)
		if err != nil {
			log.Warnf("decoding peerID failed err=%v, id=%v", err, id)
		}
		peerIDs = append(peerIDs, peerID)
	}
	nm.blockProducersMu.Lock()
	nm.blockProducers = peerIDs
	nm.blockProducersMu.Unlock()
}

func (nm *NetworkManager) getBlockProducers() []peer.ID {
	nm.blockProducersMu.RLock()
	defer nm.blockProducersMu.RUnlock()
	return nm.blockProducers
}

func (nm *NetworkManager) isBlockProducer(id peer.ID) bool {
	for _, bp := range nm.getBlockProducers() {
		if bp == id {
			return true
		}
	}
	return false
}

func (nm *NetworkManager) findBlockProducerLoop() {

	defer nm.wg.Done()
	for {
		select {
		case <-nm.quitCh:
			return
		case <-time.After(findBPInterval):
			unknownBPs := make([]string, 0)
			for _, id := range nm.getBlockProducers() {
				if len(nm.peerStore.Addrs(id)) == 0 {
					unknownBPs = append(unknownBPs, id.Pretty())
				}
			}
			nm.routingQuery(unknownBPs)
			nm.connectBlockProducers()
		}
	}

}

func (nm *NetworkManager) newStream(pid peer.ID) (libnet.Stream, error) {
	ctx, _ := context.WithTimeout(context.Background(), dialTimeout) // nolint
	return nm.host.NewStream(ctx, pid, protocolID)
}

func (nm *NetworkManager) connectBlockProducers() {
	for _, bpID := range nm.getBlockProducers() {
		if nm.isStopped() {
			return
		}

		if nm.GetNeighbor(bpID) == nil && bpID != nm.host.ID() && len(nm.peerStore.Addrs(bpID)) > 0 {
			stream, err := nm.newStream(bpID)
			if err != nil {
				log.Warnf("failed to create block producer stream pid=%s, err=%v", bpID.Pretty(), err)
				continue
			}
			nm.HandleStream(stream, outbound)
		}
	}
}

func (nm *NetworkManager) ConnectBlockProducers(ids []string) {
	nm.setBlockProducers(ids)
}

func (nm *NetworkManager) HandleStream(s libnet.Stream, direction connDirection) {
	remotePID := s.Conn().RemotePeer()

	log.Debug("swaggit handles stream from ", remotePID)
	nm.freshPeer(remotePID)

	if nm.isStreamBlack(s) {
		s.Conn().Close()
		return
	}

	peer := nm.GetNeighbor(remotePID)
	if peer != nil {
		s.Reset()
		return
	}

	if nm.NeighborCount(direction) >= nm.neighborCap[direction] {

		var p2pm p2pMessage

		if !nm.isBlockProducer(remotePID) {
			log.Infof("neighbor count exceeds, close connection. remoteID=%v, addr=%v", remotePID.Pretty(), s.Conn().RemoteMultiaddr())
			if direction == inbound {
				pid, _ := randomPID()
				bytes, _ := nm.getRoutingResponse([]string{pid.Pretty()})
				if len(bytes) > 0 {
					msg := p2pm.Encode(
						&MessagePackets{
							chainID: nm.config.ChainID,
							msgType: RoutingTableResponse, version: nm.config.Version, isCompressed: 1, isEncrypted: 0, data: bytes})
					s.Write(msg.raw())
				}
				time.AfterFunc(time.Second, func() { s.Conn().Close() })
			} else {
				s.Conn().Close()
			}
			return
		}
		nm.kickNormalNeighbors(direction)

	}
	nm.AddNeighbor(NewPeer(s, nm, direction))
}

func (nm *NetworkManager) dumpRoutingTableLoop() {
	defer nm.wg.Done()
	var lastSaveTime int64
	for {
		select {
		case <-nm.quitCh:
			return
		case <-time.After(dumpRoutingTableInterval):
			if lastSaveTime < nm.lastUpdateTime.Load() {
				nm.DumpRoutingTable()
				lastSaveTime = time.Now().Unix()
			}
		}
	}
}

func (nm *NetworkManager) syncRoutingTableLoop() {
	nm.routingQuery([]string{nm.host.ID().Pretty()})
	defer nm.wg.Done()

	for {
		select {
		case <-nm.quitCh:
			return
		case <-time.After(syncRoutingTableInterval):
			pid, _ := randomPID()
			nm.routingQuery([]string{pid.Pretty()})
		}
	}
}

func (nm *NetworkManager) metricsStatLoop() {
	defer nm.wg.Done()
	for {
		select {
		case <-nm.quitCh:
			return
		case <-time.After(metricsStatInterval):
			// TODO metrics
			//	neighborCountGauge.Set(float64(nm.AllNeighborCount()), nil)
			//	routingCountGauge.Set(float64(nm.routingTable.Size()), nil)
		}
	}
}

func (nm *NetworkManager) AllNeighborCount() int {
	nm.neighborMutex.RLock()
	defer nm.neighborMutex.RUnlock()

	return len(nm.neighbors)
}

func (nm *NetworkManager) storePeerInfo(peerID peer.ID, addrs []multiaddr.Multiaddr) {
	nm.peerStore.ClearAddrs(peerID)
	nm.peerStore.AddAddrs(peerID, addrs, peerstore.PermanentAddrTTL)
	added, err := nm.routingTable.TryAddPeer(peerID, true, false)

	if err != nil || added == false {
		log.Warnf("failed trying to add peer=%s with error=%v", peerID.Pretty(), err)
	}

	nm.lastUpdateTime.Store(time.Now().Unix())

}

func (nm *NetworkManager) deletePeerInfo(peerID peer.ID) {
	nm.peerStore.ClearAddrs(peerID)
	nm.routingTable.RemovePeer(peerID)
	nm.lastUpdateTime.Store(time.Now().Unix())

}

func (nm *NetworkManager) AddNeighbor(p *Peer) {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()

	if nm.neighbors[p.id] == nil {
		log.Debug("adding swaggit p2p neighbors ", p.id)
		p.Start()
		nm.neighbors[p.id] = p
		nm.neighborCount[p.direction]++
	}
}

func (nm *NetworkManager) RemoveNeighbor(peerID peer.ID) {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()

	p := nm.neighbors[peerID]
	if p != nil {
		log.Debug("removing swaggit p2p peer ", peerID)
		p.Stop()
		delete(nm.neighbors, peerID)
		nm.neighborCount[p.direction]--
	}
}

func (nm *NetworkManager) GetNeighbor(peerID peer.ID) *Peer {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()

	return nm.neighbors[peerID]
}

func (nm *NetworkManager) GetAllNeighbors() []*Peer {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()

	peers := make([]*Peer, 0, len(nm.neighbors))
	for _, p := range nm.neighbors {
		peers = append(peers, p)
	}
	return peers
}

func (nm *NetworkManager) CloseAllNeighbors() {
	for _, p := range nm.GetAllNeighbors() {
		p.Stop()
	}
}

func (nm *NetworkManager) AllNeighborsCount() int {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()
	return len(nm.neighbors)
}

// NeighborCount returns the neighbor amount of the given direction.
func (nm *NetworkManager) NeighborCount(direction connDirection) int {
	nm.neighborMutex.RLock()
	defer nm.neighborMutex.RUnlock()

	return nm.neighborCount[direction]
}

func (nm *NetworkManager) kickNormalNeighbors(direction connDirection) {
	nm.neighborMutex.Lock()
	defer nm.neighborMutex.Unlock()
	for _, p := range nm.neighbors {
		if nm.neighborCount[direction] < nm.neighborCap[direction] {
			return
		}
		if direction == p.direction && !nm.isBlockProducer(p.id) {
			log.Debug("kicking swaggit p2p peer... gone ", p.id)
			p.Stop()
			delete(nm.neighbors, p.id)
			nm.neighborCount[direction]--
		}
	}
}

func (nm *NetworkManager) DumpRoutingTable() {
	file, err := os.Create(filepath.Join(nm.config.DataPath, routingTableFile))
	if err != nil {
		log.Errorf("failed creating routing table file with error=%v at path=%s", err, nm.config.DataPath)
		return
	}

	defer file.Close()
	file.WriteString(fmt.Sprintf("# %s\n", time.Now().String()))
	for _, peerID := range nm.routingTable.ListPeers() {
		for _, addr := range nm.peerStore.Addrs(peerID) {
			if manet.IsPublicAddr(addr) {
				line := fmt.Sprintf("%s/ipfs/%s\n", addr.String(), peerID.Pretty())
				file.WriteString(line)
			}
		}
	}
}

func (nm *NetworkManager) LoadRoutingTable() {
	routingFile := filepath.Join(nm.config.DataPath, routingTableFile)
	if _, err := os.Stat(routingFile); err != nil {
		if os.IsNotExist(err) {
			log.Infof("no routing file. file=%v", routingFile)
			return
		}
	}
	file, err := os.Open(routingFile)
	if err != nil {
		log.Errorf("open routing file failed. err=%v, file=%v", err, routingFile)
		return
	}
	defer file.Close()
	br := bufio.NewReader(file)

	for {
		line, err := br.ReadString('\n')
		if err != nil {
			break
		}
		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}
		maddr, _ := multiaddr.NewMultiaddr(line)
		if !manet.IsPublicAddr(maddr) {
			log.Debugf("ignoring private addr %v", line)
			continue
		}
		peerID, addr, err := parseMultiaddr(line[:len(line)-1])
		if err != nil {
			log.Warnf("could not parse multiaddr err =%v, str=%v", err, line)
			continue
		}
		if peerID == nm.host.ID() {
			continue
		}
		nm.storePeerInfo(peerID, []multiaddr.Multiaddr{addr})
	}

}

func (nm *NetworkManager) routingQuery(ids []string) {
	if len(ids) == 0 {
		return
	}

	query := &netpb.RoutingQuery{Ids: ids}
	bytes, err := json.Marshal(query)
	if err != nil {
		panic(err)
	}

	nm.Broadcast(bytes, RoutingTableQuery, UrgentMessage)
	outboundNeighborCount := nm.NeighborCount(outbound)
	if outboundNeighborCount >= nm.neighborCap[outbound] {
		return
	}
	allPeerIDs := nm.routingTable.ListPeers()
	r := rand.New(rand.NewSource(time.Now().Unix()))
	perm := r.Perm(len(allPeerIDs))

	for i, t := 0, 0; i < len(perm) && t < nm.neighborCap[outbound]-outboundNeighborCount; i++ {
		if nm.isStopped() {
			return
		}

		peerID := allPeerIDs[perm[i]]
		if peerID == nm.host.ID() {
			continue
		}

		if nm.GetNeighbor(peerID) != nil {
			continue
		}

		log.Debugf("dialing peer: pid=%v", peerID.Pretty())
		stream, err := nm.newStream(peerID)
		if err != nil {
			log.Warnf("cannot create stream for pid=%s, err=%v", peerID.Pretty(), err)

			if strings.Contains(err.Error(), "connected to wrong peer") {
				nm.deletePeerInfo(peerID)
				continue
			}
			nm.recordDialFail(peerID)
			if nm.isDead(peerID) {
				nm.deletePeerInfo(peerID)
			}
			continue
		}
		nm.HandleStream(stream, outbound)
		nm.SendToPeer(peerID, bytes, RoutingTableQuery, UrgentMessage)
		t++
	}
}

func (nm *NetworkManager) parseSeeds() {
	for _, seed := range nm.config.SeedNodes {
		peerID, addr, err := parseMultiaddr(seed)
		if err != nil {
			log.Errorf("error parsing seed nodes seed=%s, err=%v", seed, err)
			continue
		}

		if madns.Matches(addr) {
			err = nm.dnsResolve(peerID, addr)
			if err != nil {
				time.AfterFunc(60*time.Second, func() {
					log.Info("retry resolve dns")
					err := nm.dnsResolve(peerID, addr)
					if err != nil {
						return
					}
				})
			}
		} else {
			nm.storePeerInfo(peerID, []multiaddr.Multiaddr{addr})
		}
	}
}

func (nm *NetworkManager) dnsResolve(peerID peer.ID, addr multiaddr.Multiaddr) error {

	resAddrs, err := madns.Resolve(context.Background(), addr)
	if err != nil {
		log.Errorf("resolve multiaddr failed. err=%v, addr=%v", err, addr)
		return err
	}
	nm.storePeerInfo(peerID, resAddrs)

	return nil
}

func (nm *NetworkManager) Broadcast(data []byte, typ MessageType, mp MessagePriority) {

	msg := newP2PMessage(nm.config.ChainID, typ, uint16(1), data)
	wg := new(sync.WaitGroup)
	for _, p := range nm.GetAllNeighbors() {
		wg.Add(1)
		go func(p *Peer) {
			log.Debug("p2p Broadcast send to ", p.id, " ", p.Addr(), " type ", typ, " priority ", mp)
			p.SendMessage(msg, mp, true)
			wg.Done()
		}(p)

	}
	wg.Wait()

}

func (nm *NetworkManager) SendToPeer(peerID peer.ID, data []byte, typ MessageType, mp MessagePriority) {
	msg := newP2PMessage(nm.config.ChainID, typ, uint16(1), data)
	peer := nm.GetNeighbor(peerID)
	if peer != nil {
		peer.SendMessage(msg, mp, false)
	}
}

func (nm *NetworkManager) Register(id string, mTyps ...MessageType) chan IncomingMessage {
	if len(mTyps) == 0 {
		return nil
	}
	c := make(chan IncomingMessage, incomingMsgChanSize)
	for _, typ := range mTyps {
		m, _ := nm.subs.LoadOrStore(typ, new(sync.Map))
		m.(*sync.Map).Store(id, c)
	}
	return c
}

func (nm *NetworkManager) Deregister(id string, mTyps ...MessageType) {
	for _, typ := range mTyps {
		if m, exist := nm.subs.Load(typ); exist {
			m.(*sync.Map).Delete(id)
		}
	}
}

func (nm *NetworkManager) getRoutingResponse(peerIDs []string) ([]byte, error) {
	queryIDs := peerIDs
	if len(queryIDs) > maxPeerQuery {
		queryIDs = queryIDs[:maxPeerQuery]
	}

	pidSet := make(map[peer.ID]struct{})
	for _, queryID := range queryIDs {
		pid, err := peer.Decode(queryID)
		if err != nil {
			log.Warnf("decode peerID failed. err=%v, id=%v", err, queryID)
			continue
		}
		peerIDs := nm.routingTable.NearestPeers(kbucket.ConvertPeerID(pid), peerResponseCount)
		for _, id := range peerIDs {
			if !nm.isDead(id) {
				pidSet[id] = struct{}{}
			}
		}

	}

	resp := &netpb.RoutingResponse{}
	for pid := range pidSet {
		info := nm.peerStore.PeerInfo(pid)
		if len(info.Addrs) > 0 {
			peerInfo := &netpb.PeerInfo{Id: info.ID.Pretty()}

			for _, addr := range info.Addrs {
				if isPublicMaddr(addr.String()) {
					peerInfo.Addrs = append(peerInfo.Addrs, addr.String())
				}
			}
			if len(peerInfo.Addrs) > maxAddrCount {
				peerInfo.Addrs = peerInfo.Addrs[:maxAddrCount]
			}

			if len(peerInfo.Addrs) > 0 {
				resp.Peers = append(resp.Peers, peerInfo)
			}

		}

	}

	selfInfo := &netpb.PeerInfo{Id: nm.host.ID().Pretty()}
	for _, addr := range nm.host.Addrs() {
		selfInfo.Addrs = append(selfInfo.Addrs, addr.String())
	}
	resp.Peers = append(resp.Peers, selfInfo)

	bytes, _ := proto.Marshal(resp)
	return bytes, nil
}

func (nm *NetworkManager) handleRoutingTableQuery(msg *p2pMessage, from peer.ID) {
	data := msg.rawData()
	query := &netpb.RoutingQuery{}
	err := json.Unmarshal(data, query)
	if err != nil {
		return
	}

	queryIDs := query.GetIds()
	bytes, _ := nm.getRoutingResponse(queryIDs)
	if len(bytes) > 0 {
		nm.SendToPeer(from, bytes, RoutingTableResponse, UrgentMessage)
	}

}

func (nm *NetworkManager) handleRoutingTableResponse(msg *p2pMessage, from peer.ID) {
	data, _ := msg.Data()
	resp := &netpb.RoutingResponse{}
	err := proto.Unmarshal(data.data, resp)
	if err != nil {
		log.Errorf("Decoding pb failed. err=%v, bytes=%v", err, data)
		return
	}
	log.Debugf("Receiving peer infos: %v, from=%v", resp, from.Pretty())

	for _, peerInfo := range resp.Peers {
		if len(peerInfo.Addrs) > 0 {
			pid, err := peer.Decode(peerInfo.Id)
			if err != nil {
				log.Warnf("Decoding peerID failed. err=%v, id=%v", err, peerInfo.Id)
				continue
			}

			if nm.isDead(pid) || nm.isPIDBlack(pid) {
				continue
			}

			if pid == nm.host.ID() {
				continue
			}

			if nm.GetNeighbor(pid) != nil && from != pid {
				continue
			}

			addrs := make([]string, 0, len(peerInfo.Addrs))
			if from != pid {
				for _, addr := range peerInfo.Addrs {
					if isPublicMaddr(addr) {
						addrs = append(addrs, addr)
					}
				}
			} else {
				var port string
				var hasPublicMaddr bool
				for _, addr := range peerInfo.Addrs {
					if isPublicMaddr(addr) {
						addrs = append(addrs, addr)
						hasPublicMaddr = true
					} else {
						port = addr[strings.LastIndex(addr, "/")+1:]
					}
				}

				if !hasPublicMaddr {
					neighbor := nm.GetNeighbor(pid)
					if neighbor != nil {
						remoteAddr := neighbor.addr.String()
						remoteListenAddr := remoteAddr[:strings.LastIndex(remoteAddr, "/")+1] + port
						addrs = append(addrs, remoteListenAddr)

					}
				}
			}

			maddrs := make([]multiaddr.Multiaddr, 0, len(addrs))
			for _, addr := range addrs {
				ma, err := multiaddr.NewMultiaddr(addr)
				if err != nil {
					log.Warnf("Parsing multiaddr failed. err=%v, addr=%v", err, addr)
					continue
				}
				maddrs = append(maddrs, ma)
			}

			if len(maddrs) > maxAddrCount {
				maddrs = maddrs[:maxAddrCount]
			}
			if len(maddrs) > 0 {
				nm.storePeerInfo(pid, maddrs)
			}
		}
	}

}

func (nm *NetworkManager) HandleMessage(msg *p2pMessage, peerID peer.ID) {
	data, err := msg.Data()
	if err != nil {
		log.Errorf("get message data failed. err=%v", err)
		return
	}

	switch msg.msgType() {
	case RoutingTableQuery:
		go nm.handleRoutingTableQuery(msg, peerID)
	case RoutingTableResponse:
		go nm.handleRoutingTableResponse(msg, peerID)
	default:
		inMsg := NewIncomingMessage(peerID, data.data, msg.msgType())
		if m, exist := nm.subs.Load(msg.msgType()); exist {
			m.(*sync.Map).Range(func(k, v interface{}) bool {
				select {
				case v.(chan IncomingMessage) <- *inMsg:
				default:
					log.Warnf("sending incoming message failed. type=%s", msg.msgType())

				}
				return true

			})
		}
	}
}

func (nm *NetworkManager) NeighborStat() map[string]interface{} {
	ret := make(map[string]interface{})

	blackIPs := make([]string, 0)
	blackPIDs := make([]string, 0)
	nm.blackListMu.RLock()
	for ip := range nm.blackListIP {
		blackIPs = append(blackIPs, ip)
	}
	for id := range nm.blackListPID {
		blackPIDs = append(blackPIDs, id)
	}
	nm.blackListMu.RUnlock()
	ret["black_ips"] = blackIPs
	ret["black_ids"] = blackPIDs

	in := make([]string, 0)
	out := make([]string, 0)

	for _, p := range nm.GetAllNeighbors() {
		addr := p.addr.String() + "/ipfs/" + p.ID()
		if p.direction == inbound {
			in = append(in, addr)
		} else {
			out = append(out, addr)
		}
	}

	ret["neighbors"] = map[string]interface{}{
		"outbound": out,
		"inbound":  in,
	}
	ret["neighbor_count"] = map[string]interface{}{
		"outbound": nm.NeighborCount(outbound),
		"inbound":  nm.NeighborCount(inbound),
	}

	ret["bp"] = nm.getBlockProducers()
	return ret
}

func (nm *NetworkManager) PutPeerToBlack(id string) {
	pid, err := peer.Decode(id)
	if err != nil {
		log.Warnf("decode peerID failed. err=%v, id=%v", err, id)
		return
	}
	nm.RemoveNeighbor(pid)
	nm.PutPIDToBlack(pid)
	nm.deletePeerInfo(pid)
}

func (nm *NetworkManager) PutPIDToBlack(pid peer.ID) {
	nm.blackListMu.Lock()
	nm.blackListPID[pid.Pretty()] = true
	nm.blackListMu.Unlock()
	for _, ma := range nm.peerStore.Addrs(pid) {
		ip := getIPFromMaddr(ma.String())
		if len(ip) > 0 {
			nm.PutIPToBlack(ip)
		}
	}

}

func (nm *NetworkManager) PutIPToBlack(ip string) {
	nm.blackListMu.Lock()
	nm.blackListIP[ip] = true
	nm.blackListMu.Unlock()
}

func (nm *NetworkManager) isStreamBlack(s libnet.Stream) bool {
	pid := s.Conn().RemotePeer()
	nm.blackListMu.RLock()
	defer nm.blackListMu.RUnlock()

	if nm.blackListPID[pid.Pretty()] {
		return true
	}
	ma := s.Conn().RemoteMultiaddr().String()
	ip := getIPFromMaddr(ma)
	return nm.blackListIP[ip]
}

func (nm *NetworkManager) isPIDBlack(pid peer.ID) bool {
	nm.blackListMu.RLock()
	defer nm.blackListMu.RUnlock()
	return nm.blackListPID[pid.Pretty()]
}

func (nm *NetworkManager) recordDialFail(pid peer.ID) {
	nm.rtMutex.Lock()
	defer nm.rtMutex.Unlock()

	nm.retryTimes[pid.Pretty()]++
}

func (nm *NetworkManager) freshPeer(pid peer.ID) {
	nm.rtMutex.Lock()
	defer nm.rtMutex.Unlock()

	delete(nm.retryTimes, pid.Pretty())
}

func (nm *NetworkManager) isDead(pid peer.ID) bool {
	nm.rtMutex.RLock()
	defer nm.rtMutex.RUnlock()

	return nm.retryTimes[pid.Pretty()] > deadPeerRetryTimes
}
