package network

import (
	"errors"
	"fmt"
	"github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/host"
	libnet "github.com/libp2p/go-libp2p-core/network"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"net"

	libp2p "github.com/libp2p/go-libp2p"
	log "github.com/sirupsen/logrus"
	mplex "github.com/whyrusleeping/go-smux-multiplex"
	"os"
	"swagg/common"
	"swagg/encryption"
)

type PeerID = peer.ID

const (
	protocolID  = "swaggitp2p/1.0"
	privKeyFile = ".keys/priv.key"
)

type Service interface {
	Start() error
	Stop()

	ID() string
	ConnectBlockProducers([]string)
	PutPeerToBlackList(string)

	Broadcast([]byte, MessageType, MessagePriority)
	SendToPeer(PeerID, []byte, MessageType, MessagePriority)
	Register(string, ...MessageType) chan IncomingMessage
	Deregister(string, ...MessageType)

	GetAllNeighbors() []*Peer
}

type NetService struct {
	*NetworkManager
	host   host.Host
	config *common.P2PConfig
}

func (n *NetService) PutPeerToBlackList(s string) {
	panic("implement me")
}

var (
	_ Service = &NetService{}
)

func NewNetService(config *common.P2PConfig) (*NetService, error) {
	ns := &NetService{
		config: config,
	}

	if err := os.MkdirAll(config.DataPath, 0755); config.DataPath != "" && err != nil {
		log.Errorf("failed to create p2p datapath, err=%v, path=%v", err, config.DataPath)
		return nil, err
	}

	key, err := encryption.GetP2PKeys(privKeyFile)
	if err != nil {
		return nil, err
	}

	h, err := ns.startHost(key, config.ListenAddr)
	if err != nil {
		log.Errorf("failed to start a host. err=%v, listenAddr=%v", err, config.ListenAddr)
		return nil, err
	}
	ns.host = h
	ns.NetworkManager = NewNetworkManager(h, config)

	// TODO admin server

	return ns, nil

}

func (n *NetService) Start() error {
	panic("implement me")
}

func (n *NetService) ID() string {
	return n.host.ID().Pretty()
}

func (n *NetService) LocalAddrs() []multiaddr.Multiaddr {
	return n.host.Addrs()
}

func (n *NetService) Stop() {}

func (n *NetService) startHost(pk crypto.PrivKey, listenAddr string) (host.Host, error) {
	tcpAddr, err := net.ResolveTCPAddr("tcp", listenAddr)
	if err != nil {
		return nil, err
	}

	if !isPortAvailable(tcpAddr.Port) {
		ErrPortUnavailable := errors.New("")
		return nil, ErrPortUnavailable
	}
	opts := []libp2p.Option{
		libp2p.Identity(pk),
		libp2p.NATPortMap(),
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/%s/tcp/%d", tcpAddr.IP, tcpAddr.Port)),
		libp2p.Muxer(protocolID, mplex.DefaultTransport),
	}
	h, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}
	h.SetStreamHandler(protocolID, n.streamHandler)
	return h, nil
}

func (n *NetService) streamHandler(s libnet.Stream) {
	n.NetworkManager.HandleStream(s, inbound)
}
