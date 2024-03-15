package speedingmvba

import (
	"crypto/sha256"
	pb "dumbo_fabric/struct"
	"fmt"

	"github.com/golang/protobuf/proto"
)

func (mvba *MVBA) sign(msg []byte) pb.Signature {

	signcontent := mvba.sigmeta.Sign(msg)

	return pb.Signature{
		ID:      int32(mvba.ID),
		Content: signcontent,
	}
}

func (mvba *MVBA) signRaw(rawmsg pb.RawMsg) pb.Signature {
	msg, err := proto.Marshal(&rawmsg)
	if err != nil {
		panic(err)
	}
	/*h := sha256.New()
	h.Write(msg)
	hashText := h.Sum(nil)*/
	signcontent := mvba.sigmeta.Sign(msg)

	return pb.Signature{
		ID:      int32(mvba.ID),
		Content: signcontent,
	}
}

func (mvba *MVBA) batchSignRaw(rawmsg pb.RawMsg, signs []pb.Signature) pb.BatchSignature {
	msg, err := proto.Marshal(&rawmsg)
	if err != nil {
		panic(err)
	}

	mems := make([]int32, len(signs))
	tmpsigns := make([][]byte, len(signs))
	for i, tmpsign := range signs {
		mems[i] = tmpsign.ID
		tmpsigns[i] = tmpsign.Content
	}

	batchsign := mvba.sigmeta.BatchSignature(tmpsigns, msg, mems)

	//signcontent := mvba.sigmeta.Sign(hashText[:])

	return pb.BatchSignature{
		Signs: batchsign,
		Mems:  mems,
	}
}

func (mvba *MVBA) batchSign(msg []byte, signs []pb.Signature) pb.BatchSignature {

	mems := make([]int32, len(signs))
	tmpsigns := make([][]byte, len(signs))
	for i, tmpsign := range signs {
		mems[i] = tmpsign.ID
		tmpsigns[i] = tmpsign.Content
	}

	batchsign := mvba.sigmeta.BatchSignature(tmpsigns, msg, mems)

	//signcontent := mvba.sigmeta.Sign(hashText[:])

	return pb.BatchSignature{
		Signs: batchsign,
		Mems:  mems,
	}
}

func (mvba *MVBA) verifySign(msg []byte, sign pb.Signature) bool {

	res := mvba.sigmeta.Verify(int(sign.ID-1), sign.Content, msg)
	return res
}

func (mvba *MVBA) verifySigns(msg []byte, batchsign pb.BatchSignature) bool {

	res := mvba.sigmeta.BatchSignatureVerify(batchsign.Signs, msg, batchsign.Mems)
	if !res {
		return false
	}

	return true
}

func (mvba *MVBA) verifySign_RawMsg(rawmsg pb.RawMsg, sign pb.Signature) bool {
	msg, err := proto.Marshal(&rawmsg)
	if err != nil {
		panic(err)
	}

	res := mvba.sigmeta.Verify(int(sign.ID-1), sign.Content, msg)
	return res
}

func (mvba *MVBA) verifySigns_RawMsg(rawmsg pb.RawMsg, batchsign pb.BatchSignature) bool {
	msg, err := proto.Marshal(&rawmsg)
	if err != nil {
		panic(err)
	}

	res := mvba.sigmeta.BatchSignatureVerify(batchsign.Signs, msg, batchsign.Mems)
	if !res {
		fmt.Println("wrong sign of verifySigns_RawMsg")
		return false
	}

	return true
}

func (mvba *MVBA) hash(msg []byte) []byte {
	h := sha256.New()
	h.Write(msg)
	return h.Sum(nil)
}
