package dumbomvba

import (
	"bytes"
	merkletree "dumbo_fabric/crypto/merkle-tree"
	mt "dumbo_fabric/crypto/merkle-tree"
	rs "dumbo_fabric/crypto/reed-solomon"
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"fmt"

	"google.golang.org/protobuf/proto"
)

type Dumbomvba struct {
	ID        int
	Round     int
	Threshold int
	Node_num  int
	Shares    [][]byte //store shares from every node

	Sigmeta cy.Signature

	DispersalMsg   chan pb.DumbomvbaMsg //type 20
	ResponseMsg    chan pb.DumbomvbaMsg //type 21
	ReconstructMsg chan pb.DumbomvbaMsg //type 22

	MsgoutCH chan pb.SendMsg

	Mypath [][]byte
	Mymsg  []byte
	Paths  [][][]byte //paths received in other nodes' dispersal instances
	Shards [][]byte   //shards received in reconstruct instance
}

func New(ID int, Round int, Threshold int, Node_num int, sigmeta cy.Signature, DispersalMsg chan pb.DumbomvbaMsg, ResponseMsg chan pb.DumbomvbaMsg, ReconstructMsg chan pb.DumbomvbaMsg, Msgout chan pb.SendMsg) Dumbomvba {
	return Dumbomvba{
		ID:             ID,
		Round:          Round,
		Threshold:      Threshold,
		Node_num:       Node_num,
		Sigmeta:        sigmeta,
		DispersalMsg:   DispersalMsg,
		ResponseMsg:    ResponseMsg,
		ReconstructMsg: ReconstructMsg,
		MsgoutCH:       Msgout,
		Paths:          make([][][]byte, Node_num),
		Shards:         make([][]byte, Node_num),
	}
}

//check input function when use dumbomvba
func Check_input(msg []byte, useless []byte, num int, sigmeta cy.Signature, useless2 [][][]pb.BCBlock) bool {
	dumbomvbamsg := pb.DumbomvbaMsg{}
	err := proto.Unmarshal(msg, &dumbomvbamsg)
	if err != nil {
		fmt.Println(err, " check_inputs,dumbomvbamsg")
	}
	signmsg := pb.DumbomvbaMsg{
		ID:     dumbomvbamsg.ID,
		Round:  dumbomvbamsg.Round,
		Type:   dumbomvbamsg.Type,
		Msglen: dumbomvbamsg.Msglen,
		Values: dumbomvbamsg.Values,
	}
	signbyte, err := proto.Marshal(&signmsg)
	if err != nil {
		fmt.Println(err, " marshal signbyte")
	}
	return sigmeta.BatchSignatureVerify(dumbomvbamsg.SS.Signs, signbyte, dumbomvbamsg.SS.Mems)

}

func (dm *Dumbomvba) Handle_Mine_Dispersal(msg []byte, out chan []byte) {
	fmt.Println("start handle mine dispersal")
	dm.Mymsg = msg
	//step 1: encode msg to n shares
	encoder := rs.New(dm.Threshold, dm.Node_num)
	shards := encoder.Encode(msg)

	//step 2: build merkle tree on shards
	merkletree, err := mt.NewTree(shards)
	if err != nil {
		panic(err)
	}
	for i := 0; i < dm.Node_num; i++ {
		path, err := merkletree.GetMerklePath(shards[i])
		if err != nil {
			panic(err)
		}
		if i+1 == dm.ID {
			dm.Mypath = path
		} else {
			msg := pb.DumbomvbaMsg{
				ID:     int32(dm.ID),
				Round:  int32(dm.Round),
				Type:   20,
				Msglen: int32(len(msg)),
				Values: path,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				panic(err)
			}
			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 4,
				Msg:  msgbyte,
			}
			dm.MsgoutCH <- sendmsg
		}

	}

	//waite for response from other node (signature on root)
	SignValue := make([][]byte, 1)
	SignValue[0] = dm.Mypath[len(dm.Mypath)-1]
	SignMsg := pb.DumbomvbaMsg{
		ID:     int32(dm.ID),
		Round:  int32(dm.Round),
		Type:   20,
		Msglen: int32(len(msg)),
		Values: SignValue,
	}
	SignByte, err := proto.Marshal(&SignMsg)
	if err != nil {
		panic(err)
	}
	count := 0
	var Signatures [][]byte
	var Mems []int32
	for {
		msg, live := <-dm.ResponseMsg
		if !live {
			return
		}
		//fmt.Println("get a response of dm from", msg.ID)
		if !dm.Sigmeta.Verify(int(msg.ID)-1, msg.Sign.Content, SignByte) {
			panic("wrong signature for root")
		} else {
			Signatures = append(Signatures, msg.Sign.Content)
			Mems = append(Mems, msg.Sign.ID)
			count++
		}
		if count > dm.Threshold {
			break
		}
	}

	//generate input for mvba
	BatchSig := dm.Sigmeta.BatchSignature(Signatures, SignByte, Mems)
	mvbainput := pb.DumbomvbaMsg{
		ID:     int32(dm.ID),
		Round:  int32(dm.Round),
		Type:   20,
		Msglen: int32(len(msg)),
		Values: SignValue,
		SS:     &pb.BatchSignature{Signs: BatchSig, Mems: Mems},
	}
	mvbainputbyte, err := proto.Marshal(&mvbainput)
	if err != nil {
		panic(err)
	}
	out <- mvbainputbyte

}

func (dm *Dumbomvba) Handle_Dispersal(close chan bool) {
	count := 0
	for {
		var msg pb.DumbomvbaMsg
		var live bool
		select {
		case <-close:
			return
		default:
			select {
			case <-close:
				return
			case msg, live = <-dm.DispersalMsg:
				if !live {
					return
				}
			}
		}
		//fmt.Println("get a shard")
		//verify path
		path := msg.Values
		if !merkletree.VerifyPath(path[len(path)-1], path, int(dm.ID)-1) {
			panic("wrong path from")
		}
		dm.Paths[msg.ID-1] = path
		count++
		//generate feedback
		SignValue := make([][]byte, 1)
		SignValue[0] = path[len(path)-1]
		SignMsg := pb.DumbomvbaMsg{
			ID:     int32(msg.ID),
			Round:  int32(dm.Round),
			Type:   20,
			Msglen: msg.Msglen,
			Values: SignValue,
		}
		SignByte, err := proto.Marshal(&SignMsg)
		if err != nil {
			panic(err)
		}
		Signature := dm.Sigmeta.Sign(SignByte)
		responsemsg := pb.DumbomvbaMsg{
			ID:    int32(dm.ID),
			Round: int32(dm.Round),
			Type:  21,
			Sign:  &pb.Signature{ID: int32(dm.ID), Content: Signature},
		}
		responsemsgbyte, err := proto.Marshal(&responsemsg)
		dm.MsgoutCH <- pb.SendMsg{int(msg.ID), 4, responsemsgbyte}
		if count == dm.Node_num-1 {
			return
		}

	}
}

func (dm *Dumbomvba) Handle_Reconstruct(ID int, root []byte, msglen int32) ([]byte, bool) {
	//reconstruct back to orign msg
	//check if contain response shard
	//if the output is from me
	if ID == dm.ID {
		//broadcast my shard and return with my orign input directly
		reconstructmsg := pb.DumbomvbaMsg{
			ID:     int32(dm.ID),
			Round:  int32(dm.Round),
			Type:   22,
			Msglen: int32(len(dm.Mymsg)),
			Values: dm.Mypath,
		}
		dm.broadcast(reconstructmsg)
		return dm.Mymsg, true

	} else {
		//isbroadcast := false
		if dm.Paths[ID-1] != nil {
			//broadcast response shard
			reconstructmsg := pb.DumbomvbaMsg{
				ID:     int32(dm.ID),
				Round:  int32(dm.Round),
				Type:   22,
				Msglen: msglen,
				Values: dm.Paths[ID-1],
			}
			dm.broadcast(reconstructmsg)
			//isbroadcast = true
		}
		//wait for enough shards to reconstruct the orign input
		count := 0
		for {
			msg, live := <-dm.ReconstructMsg
			if !live {
				return nil, false
			}
			//check path
			path := msg.Values
			if mt.VerifyPath(root, path, int(msg.ID)-1) {
				count++
				dm.Shards[msg.ID-1] = path[0]
				if count >= dm.Threshold {
					break
				}
			}

		}

		//get enough shards,reconstruct msg
		encoder := rs.New(dm.Threshold, dm.Node_num)
		input := encoder.Reconstruct(dm.Shards, int(msglen))
		//check merkle tree root
		tree, err := mt.NewTree(dm.Shards)
		if err != nil {
			panic(err)
		}
		if bytes.Equal(tree.MerkleRoot(), root) {
			return input, true
		} else {
			return nil, false
		}
	}

}

func (dm *Dumbomvba) broadcast(msg pb.DumbomvbaMsg) {
	newMsgByte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	for i := 1; i <= dm.Node_num; i++ {
		if i != dm.ID {
			dm.MsgoutCH <- pb.SendMsg{ID: i, Type: 4, Msg: newMsgByte}
		}
	}

}
