package network

import "github.com/libp2p/go-libp2p-core/host"


type P2PConfig struct {}

type Network struct {
	host host.Host
	config *P2PConfig
}