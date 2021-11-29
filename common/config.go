package common

import "go.uber.org/atomic"

type P2PConfig struct {
	DataPath   string
	AdminPort  string
	ListenAddr string
}
type PathsConfiguration struct {
	BlocksDirName         string
	BlockArchiveDirName   string
	BlockMaxFiles         int
	StateDirName          string
	ForkDirName           string
	DefaultStateSize      int32
	DefaultStateGuardSize int32
	SysAccountName        string
	NullAccountName       string
	ProducersAccountName  string
	AuthScopeName         string
	AllScopeName          string
	ActiveName            string
	OwnerName             string
	CodeRepoName          string
}

type BlockChainConfiguration struct {
	IntervalMS               int32
	IntervalUS               int32
	TimeStampEpoch           int64
	GenesisSupportedKeyTypes []string
	Percent100Mul            int
	Percent1Mul              int
	MaxBlockNetUsage         uint32
	TargetBlockNetUsage      uint32
	BaseTxNetUsage           uint32
	MaxTxLifetime            uint32
	DeferredTxExp            uint32
	MaxTxDelay               uint32
	ThreadPoolSize           uint16
	BlockCpuEffortPercent    uint32
	KyberAlgLen              uint32
	DilithiumSigLen          uint32
	DnaMutationRate          float32
	DnaPopulation            uint32
	DnaMaxTime               int64
	DnaMinTime               int64
	DnaAddDiff               int32
	MinProducers             int
	MaxProducers             int
	ProducerRepetitions      int
	BlocksPerRound           atomic.Int32
}
