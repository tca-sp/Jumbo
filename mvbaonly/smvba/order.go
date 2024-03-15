package smvba

import (
	cy "dumbo_fabric/crypto/signature"
	"dumbo_fabric/mvbaonly/smvba/dumbomvba"
	sfm "dumbo_fabric/mvbaonly/smvba/signaturefreemvba"
	speedingmvba "dumbo_fabric/mvbaonly/smvba/speedingmvba"
	speedingmvbadm "dumbo_fabric/mvbaonly/smvba/speedingmvbadm"
	pb "dumbo_fabric/struct"
	"fmt"

	"google.golang.org/protobuf/proto"
)

//****function: initial muti-shot MVBA and start it

type Order struct {
	Node_num int
	K        int

	ID        int
	Round     int // round of dumbo mvba
	MVBAround int // round of speedingmvba

	sigmeta cy.Signature

	inputCH  chan []byte //rcv msg from order_m
	outputCH chan bool   //send msg to order_m

	msginCH  chan []byte
	msgoutCH chan pb.SendMsg

	lastCommit []byte

	check_input        func([]byte, []byte, int, cy.Signature, [][][]pb.BCBlock) bool
	check_input_rbc    func([]byte, [][]int32, chan bool) bool
	check_inputs_QCagg func([]byte, []byte, int, cy.Signature) bool

	//msgCH chan pb.Msg

	//mvbaMsgCH     chan pb.Msg
	roundBufCH      chan pb.Msg
	mvba2mvbaCH     chan pb.SendMsg
	UsingDumboMVBA  bool
	dumbomvbaCH     chan []byte
	dumbomvbabuffCH chan pb.DumbomvbaMsg

	broadcasttype string
	mvbatype      string
	baCH          chan []byte
	mrbcCH        chan []byte

	//used when choose RBC
	broadcastheights [][]int32
	oldblocks        [][][]pb.BCBlock
}

func NewOrder(id int, node_num int, K int, IP string, ips []string, sigmeta cy.Signature, inputCH chan []byte, outputCH chan bool, check_input func([]byte, []byte, int, cy.Signature, [][][]pb.BCBlock) bool, check_input_rbc func([]byte, [][]int32, chan bool) bool, check_inputs_QCagg func([]byte, []byte, int, cy.Signature) bool, msgin chan []byte, msgout chan pb.SendMsg, UsingDumboMVBA bool, dumbomvbaCH chan []byte, broadcasttype string, MVBAType string, baCH chan []byte, mRBCCH chan []byte) *Order {

	newOrder := &Order{
		Node_num: node_num,
		K:        K,

		ID:        id,
		Round:     0,
		MVBAround: 0,

		sigmeta: sigmeta,

		inputCH:  inputCH,  //rcv msg from order_m
		outputCH: outputCH, //send msg to order_m
		msginCH:  msgin,
		msgoutCH: msgout,

		//msgCH: make(chan pb.Msg, 2000),

		//mvbaMsgCH:   make(chan pb.Msg, 2000),
		roundBufCH:      make(chan pb.Msg, 2000),
		mvba2mvbaCH:     make(chan pb.SendMsg, 2000),
		UsingDumboMVBA:  UsingDumboMVBA,
		dumbomvbaCH:     dumbomvbaCH,
		dumbomvbabuffCH: make(chan pb.DumbomvbaMsg, 2000),
		broadcasttype:   broadcasttype,

		mvbatype: MVBAType,
		baCH:     baCH,
		mrbcCH:   mRBCCH,

		oldblocks: make([][][]pb.BCBlock, 1),
	}
	switch broadcasttype {
	case "RBC":
		newOrder.check_input_rbc = check_input_rbc
		newOrder.broadcastheights = make([][]int32, 1)
	case "WRBC":
		newOrder.check_input_rbc = check_input_rbc
		newOrder.broadcastheights = make([][]int32, 1)
	case "CBC":
		newOrder.check_input = check_input
	case "CBC_QCagg":
		newOrder.check_inputs_QCagg = check_inputs_QCagg
	default:
		panic("wrong broadcast type")
	}

	return newOrder

}

func (order *Order) Start() {
	//go order.handle_rcvmsgCH()
	//go order.handle_msgCH()
	//go order.handle_mvba2mvbaCH()
	//go order.handle_msgoutCH()
	order.handle_rcvCH()

}

func (order *Order) handle_rcvCH() {
	if order.mvbatype == "signaturefree" {
		fmt.Println("start signature free mvba")
		order.handle_rcvCH_signaturefreemvba()
	} else if order.UsingDumboMVBA {
		fmt.Println("start dumbomvba")
		order.handle_rcvCH_dumbomvba()
	} else {
		fmt.Println("start speeding mvba")
		order.handle_rcvCH_normal()
	}

}

func (order *Order) handle_rcvCH_normal() {
	var futurebuf chan pb.Msg
	for {
		//newCH2 := make(chan speedingmvba.SendMsg, 2000)
		mvba2order := make(chan pb.Msg, 2)
		done := make(chan bool)
		input := <-order.inputCH

		fmt.Println("start a new mvba")
		//go order.handle_mvba2mvbaCH(newCH2, done)
		mvbamsgCH := make(chan pb.Msg, 2000)
		oldbuf := futurebuf
		futurebuf = make(chan pb.Msg, 2000)
		go order.handle_msgfrommvbaCH(order.Round, mvbamsgCH, done, oldbuf, futurebuf)
		mvba := speedingmvba.New_mvba(order.Node_num, order.K, order.ID, order.Round, order.msgoutCH, order.sigmeta, order.check_input, order.check_input_rbc, order.check_inputs_QCagg, order.lastCommit, input, mvba2order, mvbamsgCH, 0, order.broadcasttype, order.broadcastheights, order.oldblocks)

		mvba.Launch()

		back := <-mvba2order
		fmt.Println("done a mvba of round ", order.Round)
		//to be done: reconstruct output by dumbomvba, if fail, should launch a new mvba
		order.lastCommit = back.RawMsg.Values

		order.outputCH <- true
		//newCH := make(chan pb.Msg, 2000)
		//order.mvbaMsgCH = newCH
		close(done)
		order.Round++
		order.MVBAround++
	}
}

func (order *Order) handle_rcvCH_dumbomvba() {
	var futurebuf chan pb.Msg
	var dmfuturebuf chan pb.DumbomvbaMsg
	futurebuf = make(chan pb.Msg)
	dmfuturebuf = make(chan pb.DumbomvbaMsg)
	for {

		dmdone := make(chan bool)
		input := <-order.inputCH
		//dispersal input by dumbomvba
		dmthreshold := (order.Node_num + 2) / 3
		DispersalMsg := make(chan pb.DumbomvbaMsg, order.Node_num)
		ResponseMsg := make(chan pb.DumbomvbaMsg, order.Node_num)
		ReconstructMsg := make(chan pb.DumbomvbaMsg, order.Node_num)

		dmoldbuf := dmfuturebuf
		dmfuturebuf = make(chan pb.DumbomvbaMsg, 2000)
		go order.handle_dumbomvbamsgCH(order.Round, dmdone, DispersalMsg, ResponseMsg, ReconstructMsg, dmoldbuf, dmfuturebuf)

		dm := dumbomvba.New(order.ID, order.Round, dmthreshold, order.Node_num, order.sigmeta, DispersalMsg, ResponseMsg, ReconstructMsg, order.msgoutCH)

		go dm.Handle_Dispersal(dmdone)

		dmoutCH := make(chan []byte, 1)
		go dm.Handle_Mine_Dispersal(input, dmoutCH)

		//run mvba until get a legal output
		var back pb.Msg
		var outputbyte []byte

		for {
			//newCH2 := make(chan speedingmvba.SendMsg, 2000)
			mvba2order := make(chan pb.Msg, 2)
			done := make(chan bool)
			//start a new mvba
			fmt.Println("start a new speeding mvba")
			mvbamsgCH := make(chan pb.Msg, 2000)
			oldbuf := futurebuf
			futurebuf = make(chan pb.Msg, 2000)
			go order.handle_msgfrommvbaCH(order.MVBAround, mvbamsgCH, done, oldbuf, futurebuf)
			//go order.handle_mvba2mvbaCH(newCH2, done)
			mvba := speedingmvbadm.New_mvba(order.Node_num, order.K, order.ID, order.MVBAround, order.msgoutCH, order.sigmeta, dumbomvba.Check_input, order.check_input_rbc, order.lastCommit, dmoutCH, mvba2order, mvbamsgCH, order.Round, order.broadcasttype, order.broadcastheights, order.oldblocks)

			mvba.Launch()

			back = <-mvba2order
			outputmsg := pb.DumbomvbaMsg{}
			err := proto.Unmarshal(back.RawMsg.Values, &outputmsg)
			if err != nil {
				panic(err)
			}

			ok := true
			outputbyte, ok = dm.Handle_Reconstruct(int(outputmsg.ID), outputmsg.Values[0], outputmsg.Msglen)
			if ok {
				order.MVBAround++
				break
			} else {
				panic("reconstruct input wrong")
			}
			close(done)

		}
		fmt.Println("done a dumbo mvba of round ", order.Round)
		//to be done: reconstruct output by dumbomvba, if fail, should launch a new mvba
		order.lastCommit = outputbyte
		order.outputCH <- true
		//newCH := make(chan pb.Msg, 2000)
		//order.mvbaMsgCH = newCH
		order.Round++
		close(dmdone)

	}
}

func (order *Order) handle_rcvCH_signaturefreemvba() {
	bafuturebuf := make(chan pb.BAMsg, 1000)
	rbcfuturebuf := make(chan pb.RBCMsg, 1000)
	baoldbuf := make(chan pb.BAMsg, 1000)
	rbcoldbuf := make(chan pb.RBCMsg, 1000)
	for {
		//newCH2 := make(chan speedingmvba.SendMsg, 2000)
		mvba2order := make(chan []byte, 2)
		done := make(chan bool)

		fmt.Println("start a new signature free mvba of round", order.Round)

		checkbabuf(baoldbuf, bafuturebuf, order.Round)
		bamsgCH := make(chan pb.BAMsg, 2000)
		baoldbuf = bafuturebuf
		bafuturebuf = make(chan pb.BAMsg, 10000)
		go order.handle_bamsgCH(order.Round, done, bamsgCH, baoldbuf, bafuturebuf)

		checkrbcbuf(rbcoldbuf, rbcfuturebuf, order.Round)
		rbcmsgCH := make(chan pb.RBCMsg, 100000)
		rbcoldbuf = rbcfuturebuf
		rbcfuturebuf = make(chan pb.RBCMsg, 100000)
		go order.handle_rbcmsgCH(order.Round, done, rbcmsgCH, rbcoldbuf, rbcfuturebuf)

		input := <-order.inputCH
		mvba := sfm.New_mvba(order.Node_num, order.K, order.ID, order.Round, order.msgoutCH, bamsgCH, rbcmsgCH, order.check_input_rbc, order.lastCommit, input, mvba2order, order.broadcastheights)

		mvba.Launch()

		output := <-mvba2order
		fmt.Println("done a mvba of round ", order.Round)
		//to be done: reconstruct output by dumbomvba, if fail, should launch a new mvba
		order.lastCommit = output

		order.outputCH <- true
		//newCH := make(chan pb.Msg, 2000)
		//order.mvbaMsgCH = newCH
		close(done)
		fmt.Println("tmp buffer size:")
		fmt.Println("len of BA buffer:", len(bafuturebuf))
		fmt.Println("len of RBC buffer:", len(rbcfuturebuf))
		order.Round++
	}
}

func checkbabuf(old chan pb.BAMsg, new chan pb.BAMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.MVBARound >= int32(height) {
			new <- oldmsg
		}
	}
}
func checkrbcbuf(old chan pb.RBCMsg, new chan pb.RBCMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.Round >= int32(height) {
			new <- oldmsg
		}
	}
}

