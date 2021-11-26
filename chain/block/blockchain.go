package block

type Blockchain struct {
	Blocks []Block
	genesis string
	pubKey string
	privKey string
	info *BlockchainInfo
}

type BlockchainInfo struct {
	ID string
	Version string
	NumBlocks int
	Name int
	PeersConnected int
}


func NewBlockchain(pubHex, privHex string) *Blockchain {

	return &Blockchain{}

}

func (b *Blockchain) GetPubKey() string {
	return b.pubKey
}

func (b *Blockchain) GetPrivKey() string {
	return b.privKey
}

func (b *Blockchain) GetGenesis() string {
	return b.genesis
}

func (b *Blockchain) GetInfo() *BlockchainInfo {
	return b.info
}

