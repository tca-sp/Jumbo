package main

import (
	pb "dumbo_fabric/struct"
	"fmt"

	"github.com/golang/protobuf/proto"
)

func (order *order) handle_bamsgCH(round int, done chan bool, bamsgCH chan pb.BAMsg, oldbuf chan pb.BAMsg, futurebuf chan pb.BAMsg) {
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
			select {
			case futurebuf <- bamsg:
			default:
				fmt.Println("buffer full in handle_bamsgCH")
				futurebuf <- bamsg
			}
		} else if bamsg.MVBARound == int32(round) {
			select {
			case bamsgCH <- bamsg:
			default:
				fmt.Println("bamsgCH full for round", round)
				bamsgCH <- bamsg
			}

		}

	}
}

func (order *order) handle_rbcmsgCH(round int, done chan bool, rbcmsgCH chan pb.RBCMsg, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	for {
		var rbcmsg pb.RBCMsg
		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msg := <-order.rbcCH:
				err := proto.Unmarshal(msg, &rbcmsg)
				if err != nil {
					panic(err)
				}
			case rbcmsg = <-oldbuf:
			}

		}

		if rbcmsg.Round > int32(round) {
			select {
			case futurebuf <- rbcmsg:
			default:
				fmt.Println("buffer full in handle_rbcmsgCH")
				futurebuf <- rbcmsg
			}

		} else if rbcmsg.Round == int32(round) {
			select {
			case rbcmsgCH <- rbcmsg:
			default:
				fmt.Println("rbcmsgCH full for round", round)
				rbcmsgCH <- rbcmsg
			}

		}

	}
}

func (order *order) handle_ctrbcmsgCH(round int, done chan bool, rbcmsgCH []chan pb.RBCMsg, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	for {
		var rbcmsg pb.RBCMsg
		select {
		case <-done:
			return
		default:
			select {
			case <-done:
				return
			case msg := <-order.ctrbcCH:
				err := proto.Unmarshal(msg, &rbcmsg)
				if err != nil {
					panic(err)
				}
			case rbcmsg = <-oldbuf:
			}

		}

		if rbcmsg.Round > int32(round) {
			select {
			case futurebuf <- rbcmsg:
			default:
				fmt.Println("buffer full in handle_ctrbcmsgCH")
				futurebuf <- rbcmsg
			}
		} else if rbcmsg.Round == int32(round) {
			select {
			case rbcmsgCH[rbcmsg.Leader-1] <- rbcmsg:
			default:
				fmt.Println("ctrbcmsgCH full for round", round, "lid", rbcmsg.Leader)
				rbcmsgCH[rbcmsg.Leader-1] <- rbcmsg

			}

		}

	}
}

