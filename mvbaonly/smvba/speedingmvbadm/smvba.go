package speedingmvba

import (
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"fmt"
	"sync"
)

type MVBA struct {
	Node_num int
	K        int

	ID        int
	Roundmvba int
	Loop      int
	sendMsgCH chan pb.SendMsg

	sigmeta cy.Signature

	input       chan []byte //rcv msg from order_m
	inputlater  chan pb.Msg
	outputCH    chan pb.Msg //send msg to order_m
	msgCH       chan pb.Msg
	commitCH    chan []byte
	loopinputCH chan pb.Msg

	lastCommit []byte

	check_input     func([]byte, []byte, int, cy.Signature, [][][]pb.BCBlock) bool
	check_input_rbc func([]byte, [][]int32, chan bool) bool

	//msg channel

	//Halt_CH chan pb.Msg

	MVBA_done chan bool

	Pi_buff chan pb.Msg
	haltCH  chan pb.Msg

	LoopMsgCH  chan pb.Msg
	LoopMsgBuf chan pb.Msg
	OldProofs  []*pb.Proof
	OldCoins   []int
	MVBARoud   int

	//used when check others input(RBC)
	broadcasttype    string
	broadcastheights [][]int32
	oldblocks        [][][]pb.BCBlock
}

/*
node_num: number of parties
id: id of me (from 1 to node_num)
round: session id for this mvba instance
sendMsgCH: store msg that will be send to other mvba parties (by order)
sk: sk
pks: pks of all parties
check_input: external validity function
lastcommit: output of last mvba, use to help check_input
rcvch: receive the input of this mvba
sendch: send output of this mvba
msgCH: msgs from other mvba parties
*/
func New_mvba(node_num int, K int, id int, round int, sendMsgCH chan pb.SendMsg, sigmeta cy.Signature, check_input func([]byte, []byte, int, cy.Signature, [][][]pb.BCBlock) bool, check_input_rbc func([]byte, [][]int32, chan bool) bool, lastcommit []byte, input chan []byte, sendCH chan pb.Msg, msgCH chan pb.Msg, MVBARoud int, broadcasttype string, broadcastheights [][]int32, oldblocks [][][]pb.BCBlock) *MVBA {

	mvba := &MVBA{

		Node_num: node_num,
		K:        K,

		ID:        id,
		Roundmvba: round,
		Loop:      0,
		sendMsgCH: sendMsgCH,

		sigmeta: sigmeta,

		input:       input, //rcv msg from order
		inputlater:  make(chan pb.Msg, 2),
		outputCH:    sendCH, //send msg to order
		msgCH:       msgCH,
		commitCH:    make(chan []byte),
		loopinputCH: make(chan pb.Msg, 1),

		lastCommit: lastcommit,

		//Halt_CH: make(chan pb.Msg, 4000),

		MVBA_done: make(chan bool),

		Pi_buff:    make(chan pb.Msg, node_num),
		haltCH:     make(chan pb.Msg, node_num),
		LoopMsgCH:  make(chan pb.Msg, 4000),
		LoopMsgBuf: make(chan pb.Msg, 4000),
		MVBARoud:   MVBARoud, //round number in dumbo mvba

		broadcasttype:    broadcasttype,
		broadcastheights: broadcastheights,
		oldblocks:        oldblocks,
	}

	switch broadcasttype {
	case "RBC":
		mvba.check_input_rbc = check_input_rbc
		mvba.broadcastheights = broadcastheights
	case "WRBC":
		mvba.check_input_rbc = check_input_rbc
		mvba.broadcastheights = broadcastheights
	case "CBC":
		mvba.check_input = check_input
	case "CBC_QCagg":
		mvba.check_input = check_input
	default:
		panic("wrong broadcast type")
	}
	return mvba
}

type msgCHSet struct {
	PBC_1CH []chan pb.Msg //input
	PBC_2CH []chan pb.Msg //back sign
	PBC_3CH []chan pb.Msg //combined sign
	PBC_4CH []chan pb.Msg //pi share sign

	Pi_CH   chan pb.Msg
	CT_CH   chan pb.Msg
	PreV_CH chan pb.Msg
	Vote_CH chan pb.Msg
}

type loopInfo struct {
	round       int
	loop        int
	MVBAround   int
	inputs      [][]byte
	sigmas      []pb.BatchSignature
	pis         []pb.BatchSignature
	OK          []pb.Signature
	OK_batch    pb.BatchSignature
	Shares      []pb.Signature
	Share_batch pb.BatchSignature

	PBC_done chan bool

	coin_ready chan int

	PBC1_CH []chan pb.Msg
	PBC2_CH chan pb.Msg
	PBC3_CH []chan pb.Msg
	PBC4_CH chan pb.Msg

	Pi_count      int
	Pi_Count_Lock *sync.Mutex
	Pi_CH         chan pb.Msg
	Share_ready   chan pb.BatchSignature
	Share_CH      chan pb.Msg
	Share_CH_mine chan pb.Msg
	Prev_CH       chan pb.Msg
	Vote_CH       chan pb.Msg

	lastinput []byte
}

func (mvba *MVBA) Launch() {
	fmt.Println("start mvba ", mvba.Roundmvba, mvba.MVBARoud)
	go mvba.handle_halt(mvba.Roundmvba, mvba.MVBARoud) //deal halt msg (for we need to deal with old loop halt msg)

	/*var input []byte
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return Launch")
		return
	default:
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return Launch")
			return
		case input = <-mvba.input:
			mvba.input <- input
		}
	}

	fmt.Println("get the input")
	PBC1_rawmsg := mvba.gen_rawMsg(mvba.ID, mvba.Roundmvba, 1, input, mvba.Loop)
	inputmsg := pb.Msg{RawMsg: &PBC1_rawmsg}
	mvba.broadcast(inputmsg)*/
	//fmt.Println("broadcast input in launch:", inputmsg)

	//go mvba.handle_round_msg() //divide msg by loop

	var old_loopbuf chan pb.Msg
	var new_loopbuf chan pb.Msg
	new_loopbuf = make(chan pb.Msg, 4000)
	//var last_input []byte
	//last_input = inputmsg.RawMsg.Values
	for {
		select {
		case <-mvba.MVBA_done:
			return
		default:
		}
		fmt.Println("a new loop of mvba is running in loop ", mvba.Loop)
		loop_info := &loopInfo{
			round:         mvba.Roundmvba,
			loop:          mvba.Loop,
			MVBAround:     mvba.MVBARoud,
			inputs:        make([][]byte, mvba.Node_num),
			sigmas:        make([]pb.BatchSignature, mvba.Node_num),
			pis:           make([]pb.BatchSignature, mvba.Node_num),
			PBC_done:      make(chan bool, 4000),
			coin_ready:    make(chan int, 1),
			PBC1_CH:       make([]chan pb.Msg, 4000), //input
			PBC2_CH:       make(chan pb.Msg, 4000),   //back sign
			PBC3_CH:       make([]chan pb.Msg, 4000), //combined sign
			PBC4_CH:       make(chan pb.Msg, 4000),   //pi share sign
			Pi_count:      0,
			Pi_Count_Lock: &sync.Mutex{},
			Pi_CH:         make(chan pb.Msg, 4000),
			Share_ready:   make(chan pb.BatchSignature, 4000),
			Share_CH:      make(chan pb.Msg, 4000),
			Share_CH_mine: make(chan pb.Msg, 1),
			Prev_CH:       make(chan pb.Msg, 4000),
			Vote_CH:       make(chan pb.Msg, 4000),
			//lastinput:     last_input,
		}

		for i := 0; i < mvba.Node_num; i++ {
			loop_info.PBC1_CH[i] = make(chan pb.Msg, 4000)
			loop_info.PBC3_CH[i] = make(chan pb.Msg, 4000)
		}
		fmt.Println("start handle_loop_msg")
		old_loopbuf = new_loopbuf
		new_loopbuf = make(chan pb.Msg, 4000)
		go mvba.handle_loop_msg(old_loopbuf, new_loopbuf, loop_info, mvba.Loop) //start own PBC

		//loop_info.inputs[mvba.ID-1] = last_input

		fmt.Println("start handle_PBC")
		for i := 1; i <= mvba.Node_num; i++ {
			if i != mvba.ID {
				go mvba.handle_PBC(i, loop_info) //join other PBC
			}
		}
		go mvba.handle_mine_PBC(loop_info)

		//wait for len(pis)>2f+1
		fmt.Println("start handle_pi")
		go mvba.handle_pi(loop_info)

		//wait until share ready
		go mvba.send_share(loop_info)
		//wait coin share, if havn't sent a coin share with combined sign OK, send it
		fmt.Println("start handle_share")
		go mvba.handle_share(loop_info)
		//handle prevote and vote
		fmt.Println("start handle_finish")
		mvba.handle_finish(loop_info)
		//last_input = loop_info.lastinput

	}

}

func SafeClose(ch chan bool) {
	defer func() {
		if recover() != nil {
			// close(ch) panic occur
		}
	}()

	close(ch) // panic if ch is closed
}
