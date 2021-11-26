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
	Chain string
	Version string
	Header *BlockHeader
	Hash string
	ValidatorList []string
	Contracts map[string]string
	rw sync.RWMutex

}

type Block struct {
	*BlockImpl
	Immutable bool
	CacheKey string
}

func CreateNewBlock(b []byte) Block {

return Block{}
}
