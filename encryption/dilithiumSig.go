package encryption

import (
	"bytes"
	"errors"
	dilithium "github.com/kudelskisecurity/crystals-go/crystals-dilithium"
	"swagg/common"
)

/*
	POST-QUANTUM signature using "CRYSTAL DILITHIUM" one of the 3 finalist of the POST Quantum NIST submissions
	NIST Page: https://csrc.nist.gov/projects/post-quantum-cryptography/round-3-submissions

*/



type SigUtils struct {
	d *dilithium.Dilithium
	pk []byte
	sk []byte
}

func newSigUtils() *SigUtils {

	d := dilithium.NewDilithium3(false)
	pk, sk := d.KeyGen(nil)
	return &SigUtils{d, pk, sk,}

}

type Signature struct {
	utils *SigUtils
	Alg Algorithm
	Pub []byte
	data []byte
	Sig []byte
}



func (s *Signature) getRawPubKey() dilithium.PublicKey {

	return s.utils.d.UnpackPK(s.Pub)

}

func (s *Signature) getRawSecret() dilithium.PrivateKey {

	return s.utils.d.UnpackSK(s.utils.sk)

}

func (s *Signature) getRawSignature() (dilithium.Vec, dilithium.Vec, []byte) {
	v1, v2, rawBytes := s.utils.d.UnpackSig(s.Sig)
	return v1, v2, rawBytes
}


func (s *Signature) Verify(msg []byte) error {
	d := dilithium.NewDilithium3(false)
	v := d.Verify(s.Pub, msg, s.Sig)
	if v != true {
		return errors.New("signature is invalid")
	}
	return nil
}

func (s *Signature) ToBytes() []byte {
	se := common.NewSimpleEncoder()
	se.WriteByte(byte(s.Alg))
	se.WriteBytes(s.Sig)
	se.WriteBytes(s.Pub)
	return se.Bytes()
}



func (s *Signature) Hash() []byte {
	return common.Sha3(s.ToBytes())
}

func (s *Signature) Equal(sig *Signature) bool {
	return s.Alg == sig.Alg && bytes.Equal(s.Pub, sig.Pub) && bytes.Equal(s.Sig, sig.Sig)
}