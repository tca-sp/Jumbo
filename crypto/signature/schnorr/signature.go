package schnorr

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
	keypair cgo_sign.Keypair
}

func (sig *Signature) Sign(msg []byte) []byte {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(hashText) != 32 {
		panic("wrong length of schnorr signature")
	}
	return cgo_sign.Schnorr_Sign(sig.keypair, hashText)
}

func (sig *Signature) Verify(id int, signature []byte, msg []byte) bool {
	//fmt.Println("verify signature of ", id)
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	return cgo_sign.Schnorr_Verify(sig.Pks[id], signature, hashText) == 1
}

//unify half-aggrate schnorr signatures and simply join ecdsa and schnorr signature
func (sig *Signature) BatchSignature(signatures [][]byte, msg []byte, mems []int32) []byte {
	var batch []byte
	//fmt.Println("batch length ", len(signatures))
	for _, sig := range signatures {
		batch = append(batch, sig...)
	}
	//fmt.Println("batch signature length ", len(batch)/64)
	return batch
}

//unify half-aggrate schnorr signatures verify and simply verify ecdsa or schnorr signature one by one
func (sig *Signature) BatchSignatureVerify(BatchSignature []byte, msg []byte, mems []int32) bool {
	h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)
	if len(BatchSignature)/64 != len(mems) {
		panic("mismatch the number of signatures and signers")
	}
	for i := 0; i < len(mems); i++ {
		var signature [64]byte
		for j := 0; j < 64; j++ {
			signature[j] = BatchSignature[64*i+j]
		}
		if cgo_sign.Schnorr_Verify(sig.Pks[mems[i]-1], signature[:], hashText) != 1 {
			fmt.Println("signature of ", i, mems[i], "wrong")
			return false
		}
	}
	return true
}

//load keys
func (sig *Signature) Init(path string, num int, id int) {
	//load sk
	skpath := fmt.Sprintf("%sschnorrkey/sk/sk%d.pem", path, id-1)
	fmt.Println(id, " read sk in ", skpath)
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

	//generate keypair
	sig.keypair = cgo_sign.SchnorrGetKeypair(sig.Sk)

	//load pk
	sig.Pks = make([][]byte, num)
	for i := 0; i < num; i++ {
		pkpath := fmt.Sprintf("%sschnorrkey/pk/pk%d.pem", path, i)
		fmt.Println(id, " read pk", i, " in ", pkpath)
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
