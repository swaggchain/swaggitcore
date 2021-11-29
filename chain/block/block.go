package block

import "sync"

type IBlock interface {
	Serialize() []byte
	String() string
	VerifyHash()
	IsImmutable() bool
	Size() int64
}

type BlockImpl struct {
	IBlock
	ChainID       string
	Version       string
	Header        *BlockHeader
	Hash          string
	ValidatorList []string
	Contracts     map[string]string
	rw            sync.RWMutex
}

type Block struct {
	*BlockImpl
	Immutable bool
	CacheKey  string
}

func CreateNewBlock(bl *BlockImpl) *Block {

	b := &Block{}
	b.rw.Lock()
	b.ChainID = bl.ChainID
	b.Version = bl.Version
	b.Header = bl.Header
	b.Hash = bl.Hash
	b.ValidatorList = bl.ValidatorList
	b.Contracts = bl.Contracts
	b.rw.Unlock()
	return b

}
