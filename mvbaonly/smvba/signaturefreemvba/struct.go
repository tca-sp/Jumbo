package signaturefreemvba

import (
	pb "dumbo_fabric/struct"
)

type MVBA struct {
	Node_num int
	K        int

	ID        int
	Roundmvba int
	Roundba   int
	Loop      int

	input    []byte      //input of mvba
	outputCH chan []byte //send output to order_m

	sendMsgCH chan pb.SendMsg //send msg to other nodes
	bamsgCH   chan pb.BAMsg   //receive msg from other nodes
	rbcmsgCH  chan pb.RBCMsg
	rbcmsgCHs []chan pb.RBCMsg

	lastCommit []byte

	check_input_rbc func([]byte, [][]int32, chan bool) bool
	rbcout1         chan pb.RBCOut
	rbcout2         chan pb.RBCOut
	rbcouts         [][]byte

	MVBA_done chan bool
	closeval  chan bool

	//used when check others input(RBC)
	broadcastheights [][]int32
}
