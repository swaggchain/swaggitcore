package encryption

const (
	SIG_DEFAULT = iota
	BLOCK_SIG_PUBKEY
	TX_SIG_PUBKEY
	ENCODING_KEY
	DECODING_KEY
	GENESIS_SIG_PUBKEY
	UPDATE_SIG_PRIVKEY
)
