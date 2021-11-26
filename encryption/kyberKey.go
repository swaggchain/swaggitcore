package encryption

import (
	kyber "github.com/kudelskisecurity/crystals-go/crystals-kyber"
)

/*
	POST-QUANTUM shared secret using "CRYSTAL KYBER" one of the 3 finalist of the POST Quantum NIST submissions
	NIST Page: https://csrc.nist.gov/projects/post-quantum-cryptography/round-3-submissions
	Github: https://github.com/kudelskisecurity/crystals-go

*/


// KemProto Kyber encryption protocol structure
type KemProto struct {
	k *kyber.Kyber
	pk []byte
	sk []byte
}

func (k KemProto) KeyGen(seed []byte) ([]byte, []byte) {

	k.k = kyber.NewKyber768()
	k.pk, k.sk = k.k.KeyGen(seed)
	return k.pk, k.sk
}

/*
	NewCipher
	@param pubkey target public key
	@param data text to cipher
*/
func NewCipher(pubKey []byte, data []byte) ([]byte, []byte){

	k := kyber.NewKyber768()
	c, ss := k.Encaps(pubKey, data)
	return c, ss

}

/*
	Decipher
	@param secKey recipient secret key
	@param cipherText result of NewCipher
*/
func Decipher(secKey []byte, cipherText []byte) (decipheredData []byte) {

	k := kyber.NewKyber768()
	decipheredData = k.Decaps(secKey, cipherText)
	return

}

