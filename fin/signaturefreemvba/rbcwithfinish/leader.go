package reliablebroadcast

import (
	"bytes"
	pb "dumbo_fabric/struct"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_leader(nid int, lid int, round int, num int, input []byte, output1 chan pb.RBCOut, output2 chan pb.RBCOut, msgIn chan pb.RBCMsg, msgOut chan pb.SendMsg, close chan bool) BC_l {
	newBC_l := BC_l{
		nid:       nid,
		lid:       lid,
		num:       num,
		threshold: (num - 1) / 3,
		round:     round,

		input:   input,
		output1: output1,
		output2: output2,
		close:   close,

		msgIn:  msgIn,
		msgOut: msgOut,

		readyCH:  make(chan pb.RBCMsg, 1000),
		echoCH:   make(chan pb.RBCMsg, 1000),
		finishCH: make(chan pb.RBCMsg, 1000),

		readysignal:   make(chan bool, 2),
		output1signal: make(chan bool, 2),
		output2signal: make(chan bool, 2),
	}
	return newBC_l

}

//messages router
func (bcl *BC_l) handle_msgin() {
	//fmt.Println("leader: inside handle_msgin")
	var rbcmsg pb.RBCMsg
	for {

		rbcmsg = <-bcl.msgIn
		if rbcmsg.Round != int32(bcl.round) {
			panic("get a rbcmsg with wrong round")
		}

		//map messages by type 1:Val; 2: Ready; 3: Echo; 4: Finish
		switch rbcmsg.Type {
		case 1:
			//get a val msg
			panic("get a wrong type msg of val")
		case 2:
			//get a echo msg
			//fmt.Println("leader: get a echo msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
			bcl.echoCH <- rbcmsg

		case 3:
			//get a ready msg
			//fmt.Println("leader: get a ready msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
			bcl.readyCH <- rbcmsg

		case 4:
			//get a finish msg
			//fmt.Println("leader: get a echo msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
			bcl.finishCH <- rbcmsg

		default:
			panic("get a wrong type msg")
		}

	}

}

func (bcl *BC_l) Start() {
	//fmt.Println(bcl.nid, "start broadcast with finish leader ", bcl.nid, bcl.lid)
	go bcl.handle_msgin()
	go bcl.send_val()
	go bcl.send_echo()
	go bcl.send_ready()

	go bcl.handle_echo()
	go bcl.handle_ready()
	go bcl.handle_finish()

	//output
	outputcount := 0
	for {
		select {
		case <-bcl.close:
			return
		default:
			select {
			case <-bcl.close:
				return
			case <-bcl.output1signal:
				outputcount++
				bcl.output1 <- pb.RBCOut{bcl.input, bcl.lid}
			case <-bcl.output2signal:
				outputcount++
				bcl.output2 <- pb.RBCOut{bcl.input, bcl.lid}
			}

		}
		if outputcount >= 2 {
			return
		}
	}

}

func (bcl *BC_l) send_val() {
	//fmt.Println("leader: send val msg of height", bcl.round, "with msgout buff length ", len(bcl.msgOut))
	for i := 0; i < bcl.num; i++ {

		if i+1 != bcl.nid {
			msg := pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				Round:  int32(bcl.round),
				Type:   1,
				Root:   bcl.input,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				panic(err)
			}
			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		}
	}
	//fmt.Println("leader: send val msg of height", bcl.round, "done")

}

func (bcl *BC_l) send_echo() {
	msg := pb.RBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		Round:  int32(bcl.round),
		Type:   2,
		Root:   bcl.input,
	}
	//fmt.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	//fmt.Println("leader:send echo msg of height", bcl.round, "with msgout buff length ", len(bcl.msgOut))
	for i := 0; i < bcl.num; i++ {
		if i+1 != bcl.nid {
			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		} else {
			bcl.echoCH <- msg
		}
	}
	//fmt.Println("leader: send echo msg of height", bcl.round, "done")
}

func (bcl *BC_l) send_ready() {
	select {
	case <-bcl.close:
		return
	default:
		select {
		case <-bcl.close:
			return
		case <-bcl.readysignal:
		}
	}
	msg := pb.RBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		Round:  int32(bcl.round),
		Type:   3,
		Root:   bcl.input,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	//fmt.Println("leader: send ready msg of height", bcl.round, "with msgout buff length ", len(bcl.msgOut))
	for i := 0; i < bcl.num; i++ {
		if i+1 == bcl.nid {
			bcl.readyCH <- msg
		} else {
			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		}
	}
	//fmt.Println("leader: send ready msg of height", bcl.round, "done")
}

func (bcl *BC_l) send_finish() {
	msg := pb.RBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		Round:  int32(bcl.round),
		Type:   4,
		Root:   bcl.input,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	//fmt.Println("leader: send ready msg of height", bcl.round, "with msgout buff length ", len(bcl.msgOut))
	for i := 0; i < bcl.num; i++ {
		if i+1 == bcl.nid {
			bcl.finishCH <- msg
		} else {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 6,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		}
	}
	//fmt.Println("leader: send ready msg of height", bcl.round, "done")
}

func (bcl *BC_l) handle_echo() {
	echocount := 0
	for {
		var msg pb.RBCMsg
		select {
		case <-bcl.close:
			return
		default:
			select {
			case <-bcl.close:
				return
			case msg = <-bcl.echoCH:
			}
		}
		//check msg round, for leader, ignore old msg
		if msg.Round == int32(bcl.round) {
			if bytes.Equal(bcl.input, msg.Root) {
				echocount++
			}
		}

		if echocount == bcl.threshold*2+1 {
			bcl.readysignal <- true
		}

	}

}

func (bcl *BC_l) handle_ready() {
	readycount := 0
	for {
		var msg pb.RBCMsg
		select {
		case <-bcl.close:
			return
		default:
			select {
			case <-bcl.close:
				return
			case msg = <-bcl.readyCH:
			}
		}

		if msg.Round == int32(bcl.round) {
			//fmt.Println("leader: handle ready msg from ", msg.ID, "of height", msg.Round)
			root := msg.Root
			if bytes.Equal(root, bcl.input) {
				readycount++
				if readycount == bcl.threshold+1 {
					/*fmt.Println("get f+1 ready msg")*/
					bcl.readysignal <- true
				}
				if readycount == bcl.threshold*2 {
					/*fmt.Println("get 2f+1 ready msg")*/
					go bcl.send_finish()
					bcl.output1signal <- true
					return
				}
			}
		}
		//count times receiving with the same root, if == f+1, send ready msg

		//if ==2f+1, kill this process and wait for n-f response echo msg
	}
}

func (bcl *BC_l) handle_finish() {
	finishcount := 0
	for {
		var msg pb.RBCMsg
		select {
		case <-bcl.close:
			return
		default:
			select {
			case <-bcl.close:
				return
			case msg = <-bcl.finishCH:
			}
		}

		if msg.Round == int32(bcl.round) {
			//fmt.Println("leader: handle ready msg from ", msg.ID, "of height", msg.Round)
			root := msg.Root
			if bytes.Equal(root, bcl.input) {
				finishcount++
				if finishcount == bcl.threshold*2 {
					/*fmt.Println("get 2f+1 ready msg")*/
					bcl.output2signal <- true
					return
				}
			}
		}

	}
}

