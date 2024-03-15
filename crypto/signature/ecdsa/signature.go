package ecdsa

import (
	"crypto/sha256"
	cgo_sign "dumbo_fabric/crypto/signature"
	"encoding/pem"
	"fmt"
	"os"
)

type Signature struct {
	sk  []byte
	pks [][]byte
}

func (sig *Signature) Sign(msg []byte) []byte {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(hashText) != 32 {
		panic("wrong length of ecdsa signature")
	}
	return cgo_sign.EC_Sign(sig.sk, hashText)
}

func (sig *Signature) Verify(id int, signature []byte, msg []byte) bool {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	return cgo_sign.EC_Verify(sig.pks[id], signature, hashText) == 1
}

//unify half-aggrate schnorr signatures and simply join ecdsa and schnorr signature
func (sig *Signature) BatchSignature(signatures [][]byte, msg []byte, mems []int32) []byte {
	var batch []byte
	for _, sig := range signatures {
		batch = append(batch, sig...)
	}
	return batch
}

//unify half-aggrate schnorr signatures verify and simply verify ecdsa or schnorr signature one by one
func (sig *Signature) BatchSignatureVerify(BatchSignature []byte, msg []byte, mems []int32) bool {
	/*h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)*/
	if len(BatchSignature)/64 != len(mems) {
		fmt.Println(mems)
		fmt.Println(len(BatchSignature))
		panic("mismatch the number of signatures and signers")
	}
	for i := 0; i < len(mems); i++ {
		var signature []byte
		for j := 0; j < 64; j++ {
			signature = append(signature, BatchSignature[64*i+j])
		}
		ret := sig.Verify(int(mems[i]-1), signature, msg)
		if !ret {
			return ret
		}
		/*if ret := cgo_sign.EC_Verify(sig.pks[mems[i]-1], signature[:], hashText); ret != 1 {
			fmt.Println(ret)
			return false
		}*/
	}
	return true
}

//load keys
func (sig *Signature) Init(path string, num int, id int) {
	//load sk
	skpath := fmt.Sprintf("%s/ecdsakey/sk/sk%d.pem", path, id-1)
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
	sig.sk = block.Bytes
	file.Close()

	//load pk
	sig.pks = make([][]byte, num)
	for i := 0; i < num; i++ {
		pkpath := fmt.Sprintf("%s/ecdsakey/pk/pk%d.pem", path, i)
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
		sig.pks[i] = block.Bytes
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
