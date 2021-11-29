package currency

import (
	"go.uber.org/atomic"
	"swagg/chain/cache"
	"time"
)

type Currency struct {
	CurrencyID  string
	Name        string
	Symbol      string
	Description string
	Icon        string
	CoinID      string
	Decimals    int
	ChainID     string
	Info        *CurrencyPublicInfo
	config      *CurrConfig
	cache       cache.BaseCache
}

type Coin struct {
	Id           uint32
	prev         *Coin
	serial       string
	timestamp    int64
	Data         map[string]interface{}
	Sig          string
	ProducerAcct string
}

type CurrConfig struct {
	maxTotalCoinsAvailable uint32
	mintable               bool
	burnable               bool
	rootDna                string
	minFees                uint32
	activeFrom             time.Time
	tradableFrom           time.Time
	sellableFrom           time.Time
	stakableFrom           time.Time
	liquidityAddresses     map[string]string
	ownerAccount           string
}

type CurrencyPublicInfo struct {
	Name              string
	Symbol            string
	Description       string
	Icon              string
	URL               string
	ExplorerURL       string
	MaxAvailable      uint32
	Decimals          int
	Minted            atomic.Int32
	LastBlockReward   atomic.Uint32
	CurrentValue      atomic.Uint32
	LastMintTimestamp atomic.Int64
	PairedWith        map[string]string
	PairValue         map[string]*interface{}
	CurrencyAge       atomic.Int32
	TotalAvailable    atomic.Int32
	OwnerAddress      string
	TotalTxValue      atomic.Uint32
}
