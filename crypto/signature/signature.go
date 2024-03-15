package signature

type Signature interface {
	Init(path string, num int, id int)
	Sign(msg []byte) []byte
	//can only verify signatures that we already have pk when Init
	Verify(id int, signature []byte, msg []byte) bool
	//unify half-aggrate schnorr signatures and simply join ecdsa and schnorr signature, parameter mems is used only in half aggrate to store ids of signers(id start from 1)
	BatchSignature(signatures [][]byte, msg []byte, mems []int32) []byte
	//unify half-aggrate schnorr signatures verify and simply verify ecdsa or schnorr signature one by one
	BatchSignatureVerify(BatchSignature []byte, msg []byte, mems []int32) bool
	//when use bls, we can aggregate qcs
	QCAggregate(qcs [][]byte, msgs [][]byte, mems [][]int32) []byte
	QCAggregateVerify(qcs []byte, msgs [][]byte, mems [][]int32) bool
}
