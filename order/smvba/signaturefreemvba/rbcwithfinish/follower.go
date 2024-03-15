package reliablebroadcast

import (
	pb "dumbo_fabric/struct"
	//"fmt"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_follower(nid int, lid int, round int, num int, output1 chan pb.RBCOut, output2 chan pb.RBCOut, msgIn chan pb.RBCMsg, msgOut chan pb.SendMsg, check_input_rbc func([]byte, [][]int32, chan bool) bool, heights [][]int32, close chan bool, closeval chan bool) BC_f {

	newBC_f := BC_f{
		nid:       nid,
		lid:       lid,
		num:       num,
		threshold: (num - 1) / 3,
		round:     round,

		check_input_rbc: check_input_rbc,
		heights:         heights,

		output1:  output1,
		output2:  output2,
		close:    close,
		closeval: closeval,

		msgIn:  msgIn,
		msgOut: msgOut,

		valCH:         make(chan pb.RBCMsg, 100),
		readyCH:       make(chan pb.RBCMsg, 400),
		echoCH:        make(chan pb.RBCMsg, 400),
		finishCH:      make(chan pb.RBCMsg, 400),
		readysignal:   make(chan []byte, 2),
		output1signal: make(chan []byte, 2),
		output2signal: make(chan []byte, 2),
	}
	return newBC_f

}

//messages router
func (bcf *BC_f) handle_msgin() {
	//fmt.Println("rbc:", "inside handle_msgin")
	var rbcmsg pb.RBCMsg
	for {
		rbcmsg = <-bcf.msgIn
		if rbcmsg.Round != int32(bcf.round) {
			panic("get a rbcmsg with wrong round")
		}
		//map messages by type 1:Val; 2: Ready; 3: Echo; 4: Finish
		switch rbcmsg.Type {
		case 1:
			//get a val msg
			//fmt.Println("rbc:", "get a val msg from ", rbcmsg.ID)
			bcf.valCH <- rbcmsg
		case 2:
			//get a echo msg
			//fmt.Println("rbc:", "get a echo msg from ", rbcmsg.ID, "from leader", rbcmsg.Leader, "of height", rbcmsg.Round)
			bcf.echoCH <- rbcmsg

		case 3:
			//get a ready msg
			//fmt.Println("rbc:", "get a ready msg from ", rbcmsg.ID, "from leader", rbcmsg.Leader, "of height", rbcmsg.Round)
			bcf.readyCH <- rbcmsg

		case 4:
			//get a finish msg
			//fmt.Println("rbc:", "get a finish msg from ", rbcmsg.ID, "from leader", rbcmsg.Leader, "of height", rbcmsg.Round)
			bcf.finishCH <- rbcmsg

		default:
			panic("get a wrong type msg")
		}

	}

}

func (bcf *BC_f) Start() {
	//fmt.Println("rbc:", bcf.nid, "start broadcast follower ", bcf.nid, bcf.lid)
	go bcf.handle_msgin()

	go bcf.handle_val()
	go bcf.handle_echo()
	go bcf.handle_ready()
	go bcf.handle_finish()

	go bcf.send_ready()

	outputcount := 0
	for {
		select {
		case <-bcf.close:
			return
		default:
			select {
			case <-bcf.close:
				return
			case value := <-bcf.output1signal:
				outputcount++
				bcf.output1 <- pb.RBCOut{value, bcf.lid}
			case value := <-bcf.output2signal:
				outputcount++
				bcf.output2 <- pb.RBCOut{value, bcf.lid}
			}

		}
		if outputcount >= 2 {
			return
		}
	}

}

func (bcf *BC_f) handle_val() {
	//fmt.Println("rbc:", "inside handle_val")
	for {
		var msg pb.RBCMsg
		select {
		case <-bcf.close:
			return
		case <-bcf.closeval:
			return
		default:
			select {
			case <-bcf.close:
				return
			case <-bcf.closeval:
				return
			case msg = <-bcf.valCH:
			}
		}
		if msg.Round == int32(bcf.round) {
			if bcf.check_input_rbc(msg.Root, bcf.heights, bcf.close) {
				go bcf.send_echo(msg.Root)
			}
		}
	}
}

func (bcf *BC_f) handle_echo() {
	//fmt.Println("rbc:", "inside handle_echo")
	echomap := make(map[string]int)
	echocount := 0
	for {

		var msg pb.RBCMsg
		select {
		case <-bcf.close:
			return
		default:
			select {
			case <-bcf.close:
				return
			case msg = <-bcf.echoCH:
			}
		}

		//check msg round, for leader, ignore future and old msg
		if msg.Round == int32(bcf.round) {
			echocount++
			value := msg.Root
			v, ok := echomap[string(value)]
			if ok {
				echomap[string(value)] = v + 1
			} else {
				echomap[string(value)] = 1
			}

			if echomap[string(value)] == bcf.threshold*2+1 {
				bcf.readysignal <- value
			}
		}

		if echocount == bcf.num {
			return
		}

	}

}

func (bcf *BC_f) handle_ready() {

	//fmt.Println("rbc:", "inside handle_ready")
	readymap := make(map[string]int)
	readycount := 0
	for {
		var msg pb.RBCMsg
		select {
		case <-bcf.close:
			return
		default:
			select {
			case <-bcf.close:
				return
			case msg = <-bcf.readyCH:
			}
		}

		if msg.Round == int32(bcf.round) {
			readycount++
			value := msg.Root
			_, ok := readymap[string(value)]
			if ok {
				readymap[string(value)]++
			} else {
				readymap[string(value)] = 1
			}

			if readymap[string(value)] == bcf.threshold+1 {
				bcf.readysignal <- value
				//bcf.log.Println("ready to send ready in ready")
			}

			//if ==2f+1, kill this process and wait for n-f response echo msg
			if readymap[string(value)] == bcf.threshold*2+1 {
				bcf.send_finish(value)
				bcf.output1signal <- value
				//fmt.Println("rbc:", "ready ready for", msg.Leader)
				return
			}

		}

	}
}

func (bcf *BC_f) handle_finish() {

	//fmt.Println("rbc:", "inside handle_finish")
	finishmap := make(map[string]int)
	finishcount := 0
	for {
		var msg pb.RBCMsg
		select {
		case <-bcf.close:
			return
		default:
			select {
			case <-bcf.close:
				return
			case msg = <-bcf.finishCH:
			}
		}

		//fmt.Println("rbc:", "handle finish msg from ", msg.ID, "from leader", msg.Leader, "of height", msg.Round)
		if msg.Round == int32(bcf.round) {
			finishcount++
			value := msg.Root
			_, ok := finishmap[string(value)]
			if ok {
				finishmap[string(value)]++
			} else {
				finishmap[string(value)] = 1
			}

			//if ==2f+1, kill this process and wait for n-f response echo msg
			if finishmap[string(value)] == bcf.threshold*2+1 {
				bcf.output2signal <- value
				//fmt.Println("rbc:", "finish ready for", msg.Leader)
				return
			}

		}

	}
}

func (bcf *BC_f) send_echo(input []byte) {
	msg := pb.RBCMsg{
		ID:     int32(bcf.nid),
		Leader: int32(bcf.lid),
		Round:  int32(bcf.round),
		Type:   2,
		Root:   input,
	}
	////fmt.Println("rbc:",msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	////fmt.Println("rbc:","send echo msg of height", height)
	for i := 0; i < bcf.num; i++ {

		if i+1 != bcf.nid {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcf.msgOut <- sendmsg
		} else {
			bcf.echoCH <- msg
		}

	}
	//fmt.Println("rbc:", "send echo msg of height", bcf.round, "done")
}

func (bcf *BC_f) send_ready() {
	////fmt.Println("rbc:","send ready msg of height ", height)
	var value []byte
	select {
	case <-bcf.close:
		return
	default:
		select {
		case <-bcf.close:
			return
		case value = <-bcf.readysignal:
		}
	}

	msg := pb.RBCMsg{
		ID:     int32(bcf.nid),
		Leader: int32(bcf.lid),
		Round:  int32(bcf.round),
		Type:   3,
		Root:   value,
	}

	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}

	for i := 0; i < bcf.num; i++ {

		if i+1 == bcf.nid {
			bcf.readyCH <- msg
		} else {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcf.msgOut <- sendmsg
		}

	}
	//fmt.Println("rbc:", "send ready msg of height", bcf.round, "done")
}

func (bcf *BC_f) send_finish(value []byte) {
	msg := pb.RBCMsg{
		ID:     int32(bcf.nid),
		Leader: int32(bcf.lid),
		Round:  int32(bcf.round),
		Type:   4,
		Root:   value,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	////fmt.Println("rbc:","send ready msg of height", height)
	for i := 0; i < bcf.num; i++ {
		if i+1 == bcf.nid {
			bcf.finishCH <- msg
		} else {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcf.msgOut <- sendmsg
		}
	}
	//fmt.Println("rbc:", "send finish msg of height", bcf.round, "done")
}

