package common

import (
	"encoding/binary"
	"encoding/hex"
	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/ripemd160"
	"golang.org/x/crypto/sha3"
	"github.com/btcsuite/btcutil/base58"
	"hash/crc32"
)

func Sha3(raw []byte) []byte {
	defer func() {
		if e := recover(); e != nil {
			panic(e)
		}
	}()
	data := sha3.Sum256(raw)
	return data[:]
}

func Blake2b(raw []byte) []byte {
	new256, err := blake2b.New256(nil)
	if err != nil {
		return nil
	}
	new256.Write(raw)
	return new256.Sum(nil)

}


func Ripemd160(raw []byte) []byte {
	h := ripemd160.New()
	h.Write(raw)
	return h.Sum(nil)
}

func Base58Encode(raw []byte) string {
	return base58.Encode(raw)
}

func Base58Decode(s string) []byte {
	return base58.Decode(s)
}

func Parity(bit []byte) []byte {
	crc32q := crc32.MakeTable(crc32.Koopman)
	crc := crc32.Checksum(bit, crc32q)
	bs := make([]byte, 4)
	binary.LittleEndian.PutUint32(bs, crc)
	return bs
}

func ToHex(data []byte) string {
	return hex.EncodeToString(data)
}

func ParseHex(s string) []byte {
	d, err := hex.DecodeString(s)
	if err != nil {
		println(err)
		return nil
	}
	return d
}


