package encryption

import (
	"errors"
	dilithium "github.com/kudelskisecurity/crystals-go/crystals-dilithium"
	"swagg/common"
)

func CreateReadablePubKey(pubKey []byte) string {
	return common.Base58Encode(pubKey)
}

func DecodePubKey(pubKey string) []byte {
	return common.Base58Decode(pubKey)
}

type KeyPair struct {
	Alg Algorithm
	PubKey []byte
	PrivKey []byte
}

func CreateNewKeyPair(alg Algorithm) *KeyPair {

	k := &KeyPair{}
	k.setAlg(alg)
	switch alg {
	case Kyber:
		k.createKyber(nil)
	case Dilithium:
		k.createDilithium()
	}

	return k

}

func (kp *KeyPair) setAlg(alg Algorithm) {

	kp.Alg = alg

}

func (kp *KeyPair) createKyber(seed []byte) {
	var K KemProto
	pk, sk := K.KeyGen(seed)
	kp.PubKey = pk
	kp.PrivKey = sk
}

func (kp *KeyPair) createDilithium() {
	su := newSigUtils()
	kp.PubKey = su.pk
	kp.PrivKey = su.sk
}

func (kp *KeyPair) EncodePubKey() string {
	return common.Base58Encode(kp.PubKey)
}

func (kp *KeyPair) DecodePubKey(pk string) []byte {
	return common.Base58Decode(pk)
}

// Sign function is always using Dilithium algorithm
func (kp *KeyPair) Sign(info []byte) ([]byte, error) {
	if kp.Alg != Dilithium {
		return nil, errors.New("invalid key format for signature")
	}
	var d *dilithium.Dilithium
	return d.Sign(kp.PrivKey, info), nil
}


