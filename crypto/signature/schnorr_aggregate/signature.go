package schnorr_aggregate

import (
	"crypto/sha256"
	cgo_sign "dumbo_fabric/crypto/signature"
	"encoding/pem"
	"fmt"
	"os"
)

type Signature struct {
	Sk      []byte
	Pks     [][]byte
	Keypair cgo_sign.Keypair
}

func (sig *Signature) Sign(msg []byte) []byte {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(hashText) != 32 {
		panic("wrong length of half aggregate schnorr signature")
	}
	return cgo_sign.Schnorr_Sign(sig.Keypair, hashText)
}

func (sig *Signature) Verify(id int, signature []byte, msg []byte) bool {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	return cgo_sign.Schnorr_Verify(sig.Pks[id], signature, hashText) == 1
}

//unify half-aggrate schnorr signatures and simply join ecdsa and schnorr signature
func (sig *Signature) BatchSignature(signatures [][]byte, msg []byte, mems []int32) []byte {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(signatures) != len(mems) {
		panic("mismatch the number of signatures and signers")
	}
	joinSig := make([]byte, len(mems)*64)
	for i, sig := range signatures {
		for j := 0; j < 64; j++ {
			joinSig[i*64+j] = sig[j]
		}
	}
	Pks := make([]byte, len(mems)*32)
	for i, id := range mems {
		for j := 0; j < 32; j++ {
			Pks[i*32+j] = sig.Pks[id-1][j]
		}
	}

	return cgo_sign.Schnorr_Sign_Aggregate(joinSig, Pks, hashText, int32(len(mems)))
}

//unify half-aggrate schnorr signatures verify and simply verify ecdsa or schnorr signature one by one
func (sig *Signature) BatchSignatureVerify(BatchSignature []byte, msg []byte, mems []int32) bool {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(BatchSignature)/32-1 != len(mems) {
		fmt.Println("len of signature:", len(BatchSignature), "len of mems:", len(mems))
		panic("mismatch the number of signatures and signers")
	}
	Pks := make([]byte, len(mems)*32)
	for i, id := range mems {
		for j := 0; j < 32; j++ {
			Pks[i*32+j] = sig.Pks[id-1][j]
		}
	}
	return cgo_sign.Schnorr_Sign_Aggregate_verify(BatchSignature, Pks, hashText, int32(len(mems))) == 1

}

//load keys
func (sig *Signature) Init(path string, num int, id int) {
	//load Sk
	skpath := fmt.Sprintf("%sschnorrkey/sk/sk%d.pem", path, id-1)
	file, err := os.Open(skpath)
	if err != nil {
		panic(err)
	}
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	buf := make([]byte, info.Size())
	file.Read(buf)
	block, _ := pem.Decode(buf)
	sig.Sk = block.Bytes
	file.Close()

	//generate Keypair
	sig.Keypair = cgo_sign.SchnorrGetKeypair(sig.Sk)

	//load pk
	sig.Pks = make([][]byte, num)
	for i := 0; i < num; i++ {
		pkpath := fmt.Sprintf("%sschnorrkey/pk/pk%d.pem", path, i)
		file, err := os.Open(pkpath)
		if err != nil {
			panic(err)
		}
		info, err := file.Stat()
		if err != nil {
			panic(err)
		}
		buf := make([]byte, info.Size())
		file.Read(buf)
		block, _ := pem.Decode(buf)
		sig.Pks[i] = block.Bytes
		file.Close()
	}

}
func (sig *Signature) QCAggregate(qcs [][]byte, msgs [][]byte, mems [][]int32) []byte {
	return []byte("")
}

func (sig *Signature) QCAggregateVerify(qcs []byte, msgs [][]byte, mems [][]int32) bool {
	return true
}
func NewSignature() Signature {
	return Signature{}
}
