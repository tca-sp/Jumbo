package signaturefreemvba

import (
	"bytes"
	"crypto/sha256"
	ba "dumbo_fabric/order/smvba/signaturefreemvba/binaryagreement"
	rbc "dumbo_fabric/order/smvba/signaturefreemvba/rbcwithfinish"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"fmt"
)

func New_mvba(node_num int, K int, id int, round int, sendMsgCH chan pb.SendMsg, bamshCH chan pb.BAMsg, rbcmsgCH chan pb.RBCMsg, check_input_rbc func([]byte, [][]int32, chan bool) bool, lastcommit []byte, input []byte, outputCH chan []byte, broadcastheights [][]int32, byzfairness int) *MVBA {

	mvba := &MVBA{

		Node_num: node_num,
		K:        K,

		ID:        id,
		Roundmvba: round,
		Roundba:   0,
		Loop:      0,

		input:    input,    //rcv msg from order
		outputCH: outputCH, //send msg to order

		sendMsgCH: sendMsgCH, //protomsg out

		bamsgCH:   bamshCH,
		rbcmsgCH:  rbcmsgCH,
		rbcmsgCHs: make([]chan pb.RBCMsg, node_num),

		lastCommit: lastcommit,

		check_input_rbc: check_input_rbc,
		rbcout1:         make(chan pb.RBCOut, node_num),
		rbcout2:         make(chan pb.RBCOut, node_num),
		rbcouts:         make([][]byte, node_num),

		MVBA_done: make(chan bool),
		closeval:  make(chan bool),

		broadcastheights: broadcastheights,
		byzfairness:      byzfairness,
	}

	for i := 0; i < node_num; i++ {
		mvba.rbcmsgCHs[i] = make(chan pb.RBCMsg, 500)
	}

	return mvba
}

func (mvba *MVBA) Launch() {
	fmt.Println("start signature free mvba of height:", mvba.Roundmvba)
	//get input
	input := mvba.input
	go mvba.routerbcmsg()
	go mvba.start_rbcs(input)
	//start n modified RBC instances(one more round to muticast finish msg)

	//wait for 2f+1 RBC finish done
	finishcount := 0
	coinready := make(chan bool, 2)
	rbcready := make([]chan bool, mvba.Node_num)
	for i := 0; i < mvba.Node_num; i++ {
		rbcready[i] = make(chan bool, 10)
	}
	go func() {
		var output pb.RBCOut
		iscoinready := false
		for {
			select {
			case <-mvba.MVBA_done:
				return
			default:
				select {
				case <-mvba.MVBA_done:
					return
				case output = <-mvba.rbcout1:
				//	fmt.Println("get an output from ready from", output.ID)
				case output = <-mvba.rbcout2:
				//	fmt.Println("get an output from finish from", output.ID)
					finishcount++
				}
				//fmt.Println("get a rbc output", finishcount)
				mvba.rbcouts[output.ID-1] = output.Value
				rbcready[output.ID-1] <- true
				rbcready[output.ID-1] <- true
				if finishcount == ((mvba.Node_num-1)/3)*2+1 {
					if !iscoinready {
						coinready <- true
						iscoinready = true
						fmt.Println("ready to run ba")
					}
				}
			}

		}
	}()

	select {
	case <-mvba.MVBA_done:
		return
	default:
		select {
		case <-mvba.MVBA_done:
			return
		case <-coinready:
			close(mvba.closeval)
		}
	}

	oldbabuf := make(chan pb.BAMsg, 10000)
	newbabuf := make(chan pb.BAMsg, 10000)

	for {
		//fake leader election

		coin := mvba.common_coin()
		fmt.Println("get mvba coin", coin)
		input := false
		if mvba.rbcouts[coin-1] != nil {
			input = true
		}
		sencondinput := make(chan bool, 2)
		if !input {
			go func(channel chan bool) {
				select {
				case <-mvba.MVBA_done:
					return
				default:
					select {
					case <-mvba.MVBA_done:
						return
					case <-rbcready[coin-1]:
						channel <- true
						input = true
						return
					}
				}

			}(sencondinput)
		}
		output := make(chan bool, 2)
		//start ABA
		bamsgCH := make(chan pb.BAMsg, 1000)

		mvba.checkbuf(newbabuf, oldbabuf, mvba.Roundba)
		oldbabuf = newbabuf
		newbabuf = make(chan pb.BAMsg, 10000)
		closebaloop := make(chan bool)
		go mvba.handle_baloopmsg(newbabuf, oldbabuf, bamsgCH, mvba.Roundba, closebaloop)
		newba := ba.NewBA(mvba.Node_num, mvba.ID, mvba.Roundmvba, mvba.Roundba, input, sencondinput, output, bamsgCH, mvba.sendMsgCH)
		go newba.Launch()
		select {
		case <-mvba.MVBA_done:
			return
		default:
			select {
			case <-mvba.MVBA_done:
				return
			case output := <-output:
				if output {
					//waite for rbc done
					fmt.Println("wait for RBC", coin, " done")
					if !input {
						select {
						case <-mvba.MVBA_done:
							return
						default:
							select {
							case <-mvba.MVBA_done:
								return
							case <-rbcready[coin-1]:
							}
						}
					}
					fmt.Println("done a MVBA")
					mvba.outputCH <- mvba.rbcouts[coin-1]
					close(mvba.MVBA_done)
					return
				} else {
					close(closebaloop)
					mvba.Roundba++
				}
			}
		}
		//if output 0, continue loop; else output
	}
	//if finish corresponding RBC (with the output), multicast corresponding value
}

func (mvba *MVBA) checkbuf(newbuf chan pb.BAMsg, oldbuf chan pb.BAMsg, baround int) {
	length := len(oldbuf)
	for i := 0; i < length; i++ {
		oldmsg := <-oldbuf
		if oldmsg.BARound >= int32(baround) {
			newbuf <- oldmsg
		}
	}
}

func (mvba *MVBA) start_rbcs(input []byte) {
	go mvba.start_rbc_leader(input)

	for i := 1; i <= mvba.Node_num; i++ {
		if i != mvba.ID {
			go mvba.start_rbc_follower(i)
		}
	}

}

func (mvba *MVBA) start_rbc_leader(input []byte) {
	leader := rbc.NewBroadcast_leader(mvba.ID, mvba.ID, mvba.Roundmvba, mvba.Node_num, input, mvba.rbcout1, mvba.rbcout2, mvba.rbcmsgCHs[mvba.ID-1], mvba.sendMsgCH, mvba.MVBA_done)

	leader.Start()

}

func (mvba *MVBA) start_rbc_follower(leader int) {
	follower := rbc.NewBroadcast_follower(mvba.ID, leader, mvba.Roundmvba, mvba.Node_num, mvba.rbcout1, mvba.rbcout2, mvba.rbcmsgCHs[leader-1], mvba.sendMsgCH, mvba.check_input_rbc, mvba.broadcastheights, mvba.MVBA_done, mvba.closeval)

	follower.Start()
}

func (mvba *MVBA) common_coin() int {
	if mvba.byzfairness != 0 {
		return 1
	}
	h := sha256.New()
	h.Write(IntToBytes(mvba.Roundmvba*100 + mvba.Roundba*10 + mvba.Loop))
	hash := h.Sum(nil)

	coin := int(hash[0])

	return coin%mvba.Node_num + 1

}

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

