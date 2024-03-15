package smvba

import (
	pb "dumbo_fabric/struct"
	//"fmt"

	"github.com/golang/protobuf/proto"
)

//func (order *Order) handle_rcvmsgCH() {
//	for {
//		msgByte := <-order.msginCH
//		msg := &pb.Msg{}
//		err := proto.Unmarshal(msgByte, msg)
//		if err != nil {
//			fmt.Println(msgByte)
//			panic(err)
//		} else {
//			order.msgCH <- *msg
//			//fmt.Println("get a msg in handle_rcvmsgCH")
//		}
//
//	}
//
//}

func (order *Order) handle_msgfrommvbaCH(round int, mvbamsgCH chan pb.Msg, done chan bool, oldbuf chan pb.Msg, futurebuf chan pb.Msg) {
	for {

		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msgbyte := <-order.msginCH:
				mvbamsg := pb.Msg{}
				err := proto.Unmarshal(msgbyte, &mvbamsg)
				if err != nil {
					panic(err)
				}
				if int(mvbamsg.RawMsg.Round) > round {
					futurebuf <- mvbamsg
				} else if mvbamsg.RawMsg.Round == int32(round) {
					mvbamsgCH <- mvbamsg
				}
			case mvbamsg := <-oldbuf:
				//fmt.Println("get a mvba msg from buff")
				if int(mvbamsg.RawMsg.Round) > round {
					futurebuf <- mvbamsg
				} else if mvbamsg.RawMsg.Round == int32(round) {
					mvbamsgCH <- mvbamsg
				}
			}
		}

	}

}

func (order *Order) handle_dumbomvbamsgCH(round int, done chan bool, outCH1 chan pb.DumbomvbaMsg, outCH2 chan pb.DumbomvbaMsg, outCH3 chan pb.DumbomvbaMsg, oldbuf chan pb.DumbomvbaMsg, futurebuf chan pb.DumbomvbaMsg) {
	for {
		dmmsg := pb.DumbomvbaMsg{}
		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msg := <-order.dumbomvbaCH:
				err := proto.Unmarshal(msg, &dmmsg)
				if err != nil {
					panic(err)
				}
			case dmmsg = <-oldbuf:
			}

		}

		if dmmsg.Round > int32(round) {
			futurebuf <- dmmsg
		} else if dmmsg.Round == int32(round) {
			switch dmmsg.Type {
			case 20:
				outCH1 <- dmmsg
			case 21:
				outCH2 <- dmmsg
			case 22:
				outCH3 <- dmmsg
			default:
				panic("wrong type dumbomvba msg")
			}
		}

	}
}

func (order *Order) handle_bamsgCH(round int, done chan bool, bamsgCH chan pb.BAMsg, oldbuf chan pb.BAMsg, futurebuf chan pb.BAMsg) {
	for {
		var bamsg pb.BAMsg
		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msg := <-order.baCH:
				err := proto.Unmarshal(msg, &bamsg)
				if err != nil {
					panic(err)
				}
			case bamsg = <-oldbuf:
			}

		}

		if bamsg.MVBARound > int32(round) {
			//fmt.Println("get a future msg")
			futurebuf <- bamsg
			//fmt.Println("put future msg into buffer done")
		} else if bamsg.MVBARound == int32(round) {
			bamsgCH <- bamsg
		}

	}
}

func (order *Order) handle_rbcmsgCH(round int, done chan bool, rbcmsgCH chan pb.RBCMsg, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	for {
		var rbcmsg pb.RBCMsg
		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msg := <-order.mrbcCH:
				err := proto.Unmarshal(msg, &rbcmsg)
				if err != nil {
					panic(err)
				}
			case rbcmsg = <-oldbuf:
			}

		}

		if rbcmsg.Round > int32(round) {
			futurebuf <- rbcmsg
		} else if rbcmsg.Round == int32(round) {
			rbcmsgCH <- rbcmsg
		}

	}
}

/*func (order *Order) handle_msgCH() {
	oldRound := order.Round
	mvbaCH := order.mvbaMsgCH

	for {
		//check if round++
		tmpround := order.Round
		if tmpround > oldRound {
			//change channel
			fmt.Println("change channel")
			mvbaCH = order.mvbaMsgCH
			oldRound = tmpround
			order.check_round_buff()
			continue

		}

		msg := <-order.msgCH
		//fmt.Println("get a msg in handle_msgCH")
		if int(msg.RawMsg.Round) > tmpround {
			//buffer msg that we can't handle now, and drop old msg
			order.roundBufCH <- msg
			fmt.Println("buff msg of type ", msg.RawMsg.Type, " to round")
		} else if int(msg.RawMsg.Round) == tmpround {
			//fmt.Println("get a msg of type: ", msg.RawMsg.Type, " in round: ", msg.RawMsg.Round, " in handle_msgCH from", msg.RawMsg.ID)
			mvbaCH <- msg

		} else {
			//fmt.Println("get a lower round msg of round:", msg.RawMsg.Round, "of type:", msg.RawMsg.Type)
		}

	}

}*/

/*func (order *Order) check_round_buff() {
	j := len(order.roundBufCH)
	for i := 0; i < j; i++ {
		msg := <-order.roundBufCH
		if int(msg.RawMsg.Round) == order.Round {
			order.msgCH <- msg
		} else if int(msg.RawMsg.Round) > order.Round {
			order.roundBufCH <- msg
		}
	}
}*/

func (order *Order) handle_mvba2mvbaCH(mvba2mvbaCH chan pb.SendMsg, close chan bool) {
	for {
		select {
		case <-close:
			if len(mvba2mvbaCH) == 0 {
				return
			}
		default:
		}
		sendmsg := <-mvba2mvbaCH
		order.msgoutCH <- sendmsg
		//network.Send(order.sendPool[sendnsg.ID-1], sendnsg.Msg)
		//fmt.Println("send out a msg in handle_mvba2mvbaCH")
	}

}

/*func (order *Order) handle_mvba2mvbaCH(mvba2mvbaCH chan speedingmvba.SendMsg) {
	oldmvba2mvbaCH := order.mvba2mvbaCH
	oldround := order.Round
	for {
		if oldround != order.Round {
			oldround = order.Round
			oldmvba2mvbaCH = order.mvba2mvbaCH

			fmt.Println("handle_mvba2mvbaCH change to next mvba")
		}
		sendnsg := <-oldmvba2mvbaCH
		fmt.Println("before send out a msg in handle_mvba2mvbaCH")
		err := network.Send(order.sendPool[sendnsg.ID-1], sendnsg.Msg)
		if err != nil {
			panic(err)
		}
		//network.Send(order.sendPool[sendnsg.ID-1], sendnsg.Msg)
		fmt.Println("send out a msg in handle_mvba2mvbaCH")
	}

}*/

