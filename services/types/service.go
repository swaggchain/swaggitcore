package types

import "github.com/libp2p/go-libp2p-core/peer"

type Service interface {
	Start() error
	Stop()
	ID() string
	ConnectBlockProducers([]string)
	PutPeerToBlackList(string)
	Broadcast([]byte, string, int)
	SendToPeer(peer.ID, []byte, string, int)
	Register(string, ...string) chan interface{}
	Deregister(string, ...string)
	GetAllNeighbors() []*peer.ID

}


