package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"math/big"
	"os"
	//"fmt"
)

var skPath = "/home/flyme/Desktop/HLF_BFT/src/super_fabric/simple_hotstuff/config/sk/"
var pkPath = "/home/flyme/Desktop/HLF_BFT/src/super_fabric/simple_hotstuff/config/pk/"

func main() {
	for i := 0; i < 4; i++ {
		GenerateEccKey(i)
	}
}

//生成密钥对
func GenerateEccKey(id int) error {
	//使用ecdsa生成密钥对
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	//使用509
	private, err := x509.MarshalECPrivateKey(privateKey) //此处
	if err != nil {
		return err
	}
	//pem
	block := pem.Block{
		Type:  "esdsa private key",
		Bytes: private,
	}
	skpath := fmt.Sprintf("/home/flyme/Desktop/HLF_BFT/src/super_fabric/sk%d.pem", id)
	file, err := os.Create(skpath)
	if err != nil {
		return err
	}
	err = pem.Encode(file, &block)
	if err != nil {
		return err
	}
	file.Close()

	//处理公钥
	public := privateKey.PublicKey

	//x509序列化
	publicKey, err := x509.MarshalPKIXPublicKey(&public)
	if err != nil {
		return err
	}
	//pem
	public_block := pem.Block{
		Type:  "ecdsa public key",
		Bytes: publicKey,
	}
	pkpath := fmt.Sprintf("/home/flyme/Desktop/HLF_BFT/src/super_fabric/pk%d.pem", id)
	file, err = os.Create(pkpath)
	if err != nil {
		return err
	}
	//pem编码
	err = pem.Encode(file, &public_block)
	if err != nil {
		return err
	}
	return nil
}

//ecc签名--私钥
func EccSignature(sourceData []byte, privateKeyFilePath string) ([]byte, []byte) {
	//1，打开私钥文件，读出内容
	file, err := os.Open(privateKeyFilePath)
	if err != nil {
		panic(err)
	}
	info, err := file.Stat()
	buf := make([]byte, info.Size())
	file.Read(buf)
	//2,pem解密
	block, _ := pem.Decode(buf)
	//x509解密
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//哈希运算
	hashText := sha1.Sum(sourceData)
	//数字签名
	r, s, err := ecdsa.Sign(rand.Reader, privateKey, hashText[:])
	if err != nil {
		panic(err)
	}
	rText, err := r.MarshalText()
	if err != nil {
		panic(err)
	}
	sText, err := s.MarshalText()
	if err != nil {
		panic(err)
	}
	defer file.Close()
	return rText, sText
}

//ecc签名--私钥 with input privateKey
func Signature(sourceData []byte, privateKey ecdsa.PrivateKey) ([]byte, []byte) {

	//哈希运算
	h := sha256.New()
	h.Write(sourceData)
	hashText := h.Sum(nil)
	//数字签名
	r, s, err := ecdsa.Sign(rand.Reader, &privateKey, hashText[:])
	if err != nil {
		panic(err)
	}
	rText, err := r.MarshalText()
	if err != nil {
		panic(err)
	}
	sText, err := s.MarshalText()
	if err != nil {
		panic(err)
	}
	return rText, sText
}

//ecc认证

func EccVerify(rText, sText, sourceData []byte, publicKeyFilePath string) bool {
	//读取公钥文件
	file, err := os.Open(publicKeyFilePath)
	if err != nil {
		panic(err)
	}
	info, err := file.Stat()
	if err != nil {
		panic(err)
	}
	buf := make([]byte, info.Size())
	file.Read(buf)
	//pem解码
	block, _ := pem.Decode(buf)

	//x509
	publicStream, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		panic(err)
	}
	//接口转换成公钥
	publicKey := publicStream.(*ecdsa.PublicKey)
	hashText := sha1.Sum(sourceData)
	var r, s big.Int
	r.UnmarshalText(rText)
	s.UnmarshalText(sText)
	//认证
	res := ecdsa.Verify(publicKey, hashText[:], &r, &s)
	defer file.Close()
	return res
}

//ecc认证 with input publicKey

func Verify(rText, sText, sourceData []byte, publicKey ecdsa.PublicKey) bool {

	h := sha256.New()
	h.Write(sourceData)
	hashText := h.Sum(nil)
	var r, s big.Int
	r.UnmarshalText(rText)
	s.UnmarshalText(sText)
	//认证
	res := ecdsa.Verify(&publicKey, hashText[:], &r, &s)
	return res
}
