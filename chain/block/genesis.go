package block

import (
	"encoding/hex"
	"encoding/json"
	"github.com/btcsuite/btcutil/base58"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strconv"
	"swagg/chain/currency"
	"swagg/common"
	"swagg/encryption"
	"time"
)

const (
	NAME                 = "SwaggCore"
	VERSION              = "V0"
	PREFIX               = "SWC__V0__"
	MAX_BLOCK_PER_MINUTE = 5
	MAX_BLOCK_SIZE_BYTE  = 4096
	MIN_BLOCK_PRODUCERS  = 3
)

var MAX_TRANSACTION_DELAY, _ = time.ParseDuration("10 seconds")

type GenesisBlock struct {
	ComputedChainID  []byte
	InitialTimestamp string `json:"genesis_time"`
	InitialKey       string `json:"pubkey"`
	InitialSigKey    string `json:"sigkey"`
	config           *GenesisConfig
	Hash             string
}

// GenesisConfig is the decoded version of the genesis configuration
type GenesisConfig struct {
	FriendlyName        string `json:"chain_name"`
	Version             string `json:"chain_version"`
	Prefix              string `json:"chain_prefix"`
	MaxBlocksPerMinute  int32
	MaxTransactionDelay int32
	MaxBlockSize        int32
	MinBlockProducers   int32
	Producers           []string
	Currency            *currency.Currency
	StakableCurrency    *currency.Currency
	OwnerInfo           string
}

func (g *GenesisBlock) computeChainID() {

	toSerialize := g.InitialTimestamp + string(g.InitialKey) + string(g.InitialSigKey) + g.config.FriendlyName + g.config.Version + g.config.OwnerInfo
	ser, err := json.Marshal(toSerialize)
	if err != nil {
		log.Fatal("cannot compute genesis block", err)
	}
	g.ComputedChainID = common.Sha3(ser)
}

func (g *GenesisBlock) GetChainID() string {
	return hex.EncodeToString(g.ComputedChainID)
}

func (g *GenesisBlock) generateInitialKey() []byte {
	kp := encryption.CreateNewKeyPair(encryption.Kyber)
	pubKey := kp.EncodePubKey()
	privKey := kp.PrivKey
	writePrivateKey(".swagghome/.priv.key", privKey)
	g.InitialKey = pubKey
	return privKey
}

func (g *GenesisBlock) generateInitialSigKey() []byte {
	sk := encryption.CreateNewKeyPair(encryption.Dilithium)
	pubKey := sk.EncodePubKey()

	g.InitialSigKey = pubKey
	privKey := sk.PrivKey
	writePrivateKey(".swagghome/.priv_sig.key", privKey)

	return privKey
}

func (g *GenesisBlock) setConfig() {
	g.config = &GenesisConfig{}
	g.config.MaxBlockSize = MAX_BLOCK_SIZE_BYTE
	g.config.MaxBlocksPerMinute = MAX_BLOCK_PER_MINUTE
	g.config.MaxTransactionDelay = int32(MAX_TRANSACTION_DELAY.Nanoseconds())
	g.config.FriendlyName = NAME
	g.config.Version = VERSION
	g.config.Prefix = PREFIX
	g.config.MinBlockProducers = MIN_BLOCK_PRODUCERS
	g.config.Producers = make([]string, 0)
	g.config.Currency = &currency.Currency{}
	g.config.StakableCurrency = &currency.Currency{}
	g.config.OwnerInfo = base58.Encode(common.Sha3(nil))
}

func GetOrCreateGenesisBlock(path string, force bool) *GenesisBlock {

	var Gen GenesisBlock

	gb, err := ioutil.ReadFile(path + "/genesis.json")
	if err == os.ErrNotExist || force == true {
		//we create the genesis block
		log.Info("generating genesis block...")
		g := &GenesisBlock{}
		g.InitialTimestamp = strconv.FormatInt(time.Now().UnixNano(), 10)
		g.generateInitialKey()
		g.generateInitialSigKey()
		g.setConfig()
		g.computeChainID()
		g.Hash = hex.EncodeToString(common.Sha3(g.ComputedChainID))
		return g
	}

	if err == nil {
		err := json.Unmarshal(gb, &Gen)
		if err != nil {
			log.Fatal(err)
		}
		return &Gen
	}

	return nil

}

func writePrivateKey(path string, data []byte) {

	err := ioutil.WriteFile(path, data, 0644)
	if err != nil {
		log.Fatal("error writting private key file", err)
	}

}
