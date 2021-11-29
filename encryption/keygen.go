package encryption

import (
	"crypto/rand"
	"errors"
	dilithium "github.com/kudelskisecurity/crystals-go/crystals-dilithium"
	"github.com/libp2p/go-libp2p-core/crypto"
	"os"
	"swagg/common"
)

func CreateReadablePubKey(pubKey []byte) string {
	return common.Base58Encode(pubKey)
}

func DecodePubKey(pubKey string) []byte {
	return common.Base58Decode(pubKey)
}

type KeyPair struct {
	Alg     Algorithm
	PubKey  []byte
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
	case P2P:
		k.generateP2PKeys()
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

func (kp *KeyPair) generateP2PKeys() {

	privKey, pubKey, _ := crypto.GenerateEd25519Key(rand.Reader)
	pubB, _ := pubKey.Raw()
	privB, _ := privKey.Raw()
	kp.PubKey = pubB
	kp.PrivKey = privB

}

func (kp *KeyPair) P2PPrivKeyToString() string {
	return crypto.ConfigEncodeKey(kp.PrivKey)
}

func (kp *KeyPair) P2PPubKeyToString() string {
	return string(kp.PubKey)
}

func (kp *KeyPair) DecodeP2PPrivKey(data string) (crypto.PrivKey, error) {
	bytes, err := crypto.ConfigDecodeKey(data)
	if err != nil {
		return nil, err
	}
	return crypto.UnmarshalPrivateKey(bytes)
}

func (kp *KeyPair) getP2PKeyFromFile(path string) (crypto.PrivKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	kp.PrivKey = data
	return kp.DecodeP2PPrivKey(string(data))
}

func (kp *KeyPair) writeP2PKeyToFile(path string) error {

	return os.WriteFile(path, []byte(kp.P2PPrivKeyToString()), 0666)
}

func GetP2PKeys(path string) (crypto.PrivKey, error) {

	k := new(KeyPair)
	key, err := k.getP2PKeyFromFile(path)
	if err == nil {
		return key, nil
	}
	k.generateP2PKeys()

	if path != "" {

		err = k.writeP2PKeyToFile(path)
		if err != nil {
			return nil, err
		}

	}
	return k.DecodeP2PPrivKey(string(k.PrivKey))

}
