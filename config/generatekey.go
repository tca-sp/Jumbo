package main

import (
//	cgo_sign "dumbo_fabric/crypto/signature"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/bls-go-binary/bls"
)

const max_num = 500
const keypath = "/src/dumbo_fabric/config"

func main() {
	gopath := os.Getenv("GOPATH")
	blsinit()
	for i := 0; i < max_num; i++ {
/*		//generate ecdsa key
		skpath := fmt.Sprintf("%s%s/ecdsakey/sk/sk%d.pem", gopath, keypath, i)
		pkpath := fmt.Sprintf("%s%s/ecdsakey/pk/pk%d.pem", gopath, keypath, i)
		sk := cgo_sign.Generate_Key()
		signkey := cgo_sign.NewSignKey(sk, 2)
		pk := signkey.Pk
		cgo_sign.KeyStore(sk, "esdsa private key", skpath)
		cgo_sign.KeyStore(pk, "esdsa public key", pkpath)

		//generate schnorr key
		skpath = fmt.Sprintf("%s%s/schnorrkey/sk/sk%d.pem", gopath, keypath, i)
		pkpath = fmt.Sprintf("%s%s/schnorrkey/pk/pk%d.pem", gopath, keypath, i)
		sk = cgo_sign.Generate_Key()
		signkey2 := cgo_sign.NewSignKey(sk, 1)
		pk = signkey2.Pk
		cgo_sign.KeyStore(sk, "esdsa private key", skpath)
		cgo_sign.KeyStore(pk, "esdsa public key", pkpath)
*/
		//generate bls key
		skpath := fmt.Sprintf("%s%s/blskey/sk/12381/sk%d.pem", gopath, keypath, i)
		pkpath := fmt.Sprintf("%s%s/blskey/pk/12381/pk%d.pem", gopath, keypath, i)
		var blssk bls.SecretKey
		blssk.SetByCSPRNG()
		blspk := *blssk.GetPublicKey()
		skbyte := []byte(blssk.GetHexString())
		BLSKeyStore(skbyte, "bls private key", skpath)
		pkbyte := []byte(blspk.GetHexString())
		BLSKeyStore(pkbyte, "bls public key", pkpath)
	}

}

func blsinit() {
	var g_Qcoeff []uint64
	bls.Init(bls.BLS12_381)
	n := bls.GetUint64NumToPrecompute()
	g_Qcoeff = make([]uint64, n)
	var Q bls.PublicKey
	bls.BlsGetGeneratorOfPublicKey(&Q)
	bls.PrecomputeG2(g_Qcoeff, bls.CastFromPublicKey(&Q))
}

func BLSKeyStore(key []byte, keytype string, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	block := &pem.Block{
		Type:  keytype,
		Bytes: key,
	}
	err = pem.Encode(file, block)
	if err != nil {
		return err
	}
	file.Close()
	return nil
}

