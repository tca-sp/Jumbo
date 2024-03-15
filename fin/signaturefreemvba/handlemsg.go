package signaturefreemvba

import (
	pb "dumbo_fabric/struct"
//	"fmt"
)

func (mvba *MVBA) routerbcmsg() {
	for {
		select {
		case <-mvba.MVBA_done:
			return
		default:
			select {
			case <-mvba.MVBA_done:
				return
			case msg := <-mvba.rbcmsgCH:
				mvba.rbcmsgCHs[msg.Leader-1] <- msg
			}
		}
	}
}

func (mvba *MVBA) handle_baloopmsg(newbuf chan pb.BAMsg, oldbuf chan pb.BAMsg, msgbuf chan pb.BAMsg, baround int, close chan bool) {
	var msg pb.BAMsg
	for {
		select {
		case <-mvba.MVBA_done:
			return
		case <-close:
			return
		default:
			select {
			case <-mvba.MVBA_done:
				return
			case <-close:
				return
			case msg = <-oldbuf:
			default:
				select {
				case <-mvba.MVBA_done:
					return
				case <-close:
					return
				case msg = <-oldbuf:
				case msg = <-mvba.bamsgCH:
				}
			}
		}
		if msg.BARound == int32(baround) {
			msgbuf <- msg
		} else if msg.BARound > int32(baround) {
//			fmt.Println("sfmvba: get a future msg from", msg.ID, "of height", msg.MVBARound, "of round", msg.BARound)
			newbuf <- msg
		}
	}

}
