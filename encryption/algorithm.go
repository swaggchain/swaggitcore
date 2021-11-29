package encryption

type Algorithm uint8

const (
	_ Algorithm = iota
	Kyber
	Dilithium
	Secp256k1
	Ed25519
	RSA
	P2P
)

func (a Algorithm) String() string {
	switch a {
	case Kyber:
		return "crystal-kyber"
	case Dilithium:
		return "crystal-dilithium"
	case Secp256k1:
		return "secp256k1"
	case Ed25519:
		return "ed25519"
	case RSA:
		return "rsa"
	case P2P:
		return "p2p"

	default:
		return "crystal-kyber"
	}

}
