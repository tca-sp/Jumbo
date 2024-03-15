package speedingmvba

import (
	"bytes"
	pb "dumbo_fabric/struct"
	"fmt"
	"math/big"

	"github.com/golang/protobuf/proto"
)

/*type SendMsg struct {
	ID   int
	Type int
	Msg  []byte
}*/

//send msg to id
func (mvba *MVBA) send(msg pb.Msg, id int) {
	newMsgByte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	mvba.sendMsgCH <- pb.SendMsg{ID: id, Type: 1, Msg: newMsgByte}
}

func (mvba *MVBA) broadcast(msg pb.Msg) {
	newMsgByte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	for i := 1; i <= mvba.Node_num; i++ {
		if i != mvba.ID {
			mvba.sendMsgCH <- pb.SendMsg{ID: i, Type: 1, Msg: newMsgByte}
		}
	}
	//**11 fmt.Println("send msg in broadcast")
}

func (mvba *MVBA) check_values(msg *pb.Msg, round int, loop int, mvbaround int) bool {
	if loop == 0 {
		return true
	}

	oldproof := msg.OldProof
	count := 0
	if oldproof[0].Type {
		oldloop := loop - len(oldproof)
		oldcoin := mvba.OldCoins[oldloop]
		//check sigmas
		PBC2_RawMsg := pb.RawMsg{
			ID:     int32(oldcoin + 1),
			Round:  int32(round),
			Type:   2,
			Values: mvba.hash(msg.RawMsg.Values),
			Loop:   int32(oldloop),
		}
		if !mvba.verifySigns_RawMsg(PBC2_RawMsg, *oldproof[0].BatchSigns) {
			panic("wrong sigmas for check value")
			return false
		}
		count = 1

	} else {
		if len(oldproof) != loop {
			return false
		}
	}

	for i := count; i < len(oldproof); i++ {
		if oldproof[i].Type {
			return false
		}
		oldloop := loop - len(oldproof) + i
		if !mvba.verifySigns([]byte(fmt.Sprintf("%d%d%d%s", round, oldloop, mvbaround, "voteno")), *oldproof[i].BatchSigns) {
			return false
		}
	}

	return true

}

func (mvba *MVBA) handle_loop_msg(oldbuf chan pb.Msg, newbuf chan pb.Msg, loop_info *loopInfo, loop int) {

	for {
		var msg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_loop_msg")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_loop_msg")
				return
			case msg = <-mvba.msgCH:
			case msg = <-oldbuf:
			}

		}
		//**11 fmt.Println("get a msg of type: ", msg.RawMsg.Type, " in handle_loop_msg")
		if msg.RawMsg.Round < int32(loop_info.round) {
			continue
		}

		if msg.RawMsg.Type == int32(11) {
			//**11 fmt.Println("get a msg of halt")
			mvba.haltCH <- msg
			continue
		} else if msg.RawMsg.Loop > int32(loop) {
			//**11 fmt.Println("buff a msg of type:", msg.RawMsg.Type, "to loopmsgbuf")
			newbuf <- msg
			continue
		} else if msg.RawMsg.Loop < int32(loop) {
			//**11 fmt.Println("get a msg of lower loop")
			continue
		}

		switch msg.RawMsg.Type {
		case 1:
			loop_info.PBC1_CH[msg.RawMsg.ID-1] <- msg
		case 2:
			loop_info.PBC2_CH <- msg
		case 3:
			loop_info.PBC3_CH[msg.RawMsg.ID-1] <- msg
		case 4:
			loop_info.PBC4_CH <- msg
		case 5:
			loop_info.Pi_CH <- msg
		case 6:
			loop_info.Share_CH <- msg
		case 7:
			loop_info.Prev_CH <- msg
		case 8:
			loop_info.Prev_CH <- msg
		case 9:
			loop_info.Vote_CH <- msg
		case 10:
			loop_info.Vote_CH <- msg
		default:
			panic("receive a msg of wrong type in handle_loop_msg")
		}

	}
}

func (mvba *MVBA) handle_PBC(leader int, loop_info *loopInfo) {

	var PBC1msg pb.Msg
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_PBC")
		return
	default:
		select {
		case PBC1msg = <-loop_info.PBC1_CH[leader-1]:
			break
		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_PBC")
			return
		}
	}

	//**11 fmt.Println("get a PBC1msg from:", PBC1msg.RawMsg.ID)
	if PBC1msg.RawMsg.Loop < int32(loop_info.loop) {
		fmt.Println("get a old PBC1 msg from", leader)
		return
	} else if PBC1msg.RawMsg.Loop > int32(loop_info.loop) {
		fmt.Println("get a future PBC1 msg from", leader)
		return
	}
	//check msg
	switch mvba.broadcasttype {
	case "RBC":

		res := mvba.check_input_rbc(PBC1msg.RawMsg.Values, mvba.broadcastheights, loop_info.PBC_done)
		if !res {
			return
		}
	case "WRBC":

		res := mvba.check_input_rbc(PBC1msg.RawMsg.Values, mvba.broadcastheights, loop_info.PBC_done)
		if !res {
			return
		}
	case "CBC":
		res := mvba.check_input(PBC1msg.RawMsg.Values, mvba.lastCommit, mvba.Node_num*mvba.K, mvba.sigmeta, mvba.oldblocks)
		if !res {
			panic("wrong input in PBC1")
			return
		}
	case "CBC_QCagg":
		res := mvba.check_input(PBC1msg.RawMsg.Values, mvba.lastCommit, mvba.Node_num*mvba.K, mvba.sigmeta, mvba.oldblocks)
		if !res {
			panic("wrong input in PBC1")
			return
		}
	default:
		panic("wrong broadcast type")
	}

	//check old proof
	if PBC1msg.RawMsg.Loop > 0 {
		if !mvba.check_values(&PBC1msg, loop_info.round, loop_info.loop, loop_info.MVBAround) {
			panic("wrong proof for input")
			return
		}
	}
	loop_info.inputs[leader-1] = PBC1msg.RawMsg.Values

	//generate PBC2 msg and send it
	PBC1msgHash := mvba.hash(PBC1msg.RawMsg.Values)
	PBC2_RawMsg := pb.RawMsg{
		ID:     int32(leader),
		Round:  int32(loop_info.round),
		Type:   2,
		Values: PBC1msgHash,
		Loop:   int32(loop_info.loop),
	}
	PBC2_sign := mvba.signRaw(PBC2_RawMsg)
	PBC2msg := pb.Msg{
		RawMsg: &PBC2_RawMsg,
		Sign:   &PBC2_sign,
	}
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_PBC")
		return
	default:
		select {
		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			return
		default:
		}
	}
	//**11 fmt.Println("send PBC2 msg in handle PBC to:", leader)
	mvba.send(PBC2msg, leader)

	//wait PBC3 msg
	var PBC3msg pb.Msg
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_PBC")
		return
	default:
		select {
		case PBC3msg = <-loop_info.PBC3_CH[leader-1]:
			break
		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			return
		}
	}
	//**11 fmt.Println("get a PBC3msg from:", PBC3msg.RawMsg.ID)
	if PBC3msg.RawMsg.Loop < int32(loop_info.loop) {
		fmt.Println("get a old PBC3 msg from", leader)
		return
	} else if PBC3msg.RawMsg.Loop > int32(loop_info.loop) {
		fmt.Println("get a future PBC3 msg from", leader)
		return
	}
	//check PBC3 msg
	if !bytes.Equal(PBC1msgHash, PBC3msg.RawMsg.Values) {
		panic("wrong PBC3 msg")
		return
	}

	//check msg combinesigns
	res := mvba.verifySigns_RawMsg(PBC2_RawMsg, *PBC3msg.SS)
	if !res {
		panic(fmt.Sprintf("wrong combine signs for PBC3 msg of loop %d", PBC3msg.RawMsg.Loop))
		return
	}

	loop_info.sigmas[leader-1] = *PBC3msg.SS

	//generate PBC4 msg and send it
	PBC4_RawMsg := pb.RawMsg{
		ID:     int32(leader),
		Round:  int32(loop_info.round),
		Type:   4,
		Values: PBC1msgHash,
		Loop:   int32(loop_info.loop),
	}
	PBC4_sign := mvba.signRaw(PBC4_RawMsg)
	PBC4msg := pb.Msg{
		RawMsg: &PBC4_RawMsg,
		Sign:   &PBC4_sign,
	}
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_PBC")
		return
	default:
		select {
		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			return
		default:
		}
	}
	//**11 fmt.Println("send PBC4 msg in handle PBC to:", leader)
	mvba.send(PBC4msg, leader)
}

func (mvba *MVBA) handle_mine_PBC(loop_info *loopInfo) {
	if loop_info.loop == 0 {
		var input []byte
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
		PBC1_rawmsg := mvba.gen_rawMsg(mvba.ID, mvba.Roundmvba, 1, input, loop_info.loop)
		inputmsg := pb.Msg{RawMsg: &PBC1_rawmsg}
		loop_info.inputs[mvba.ID-1] = input
		mvba.broadcast(inputmsg)
	} else {
		inputmsg := <-mvba.inputlater
		if inputmsg.RawMsg.Values == nil {
			var input []byte
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
			loop_info.inputs[mvba.ID-1] = input
			inputmsg.RawMsg.Values = input
			mvba.broadcast(inputmsg)
		} else {
			loop_info.inputs[mvba.ID-1] = inputmsg.RawMsg.Values
			//fix bug
			<-mvba.input
			mvba.input <- inputmsg.RawMsg.Values
			mvba.broadcast(inputmsg)
		}

	}

	max := mvba.Node_num * 2 / 3
	//wait PBC2 msg
	PBC1msgHash := mvba.hash(loop_info.inputs[mvba.ID-1])
	PBC2_count := 0
	PBC2_RawMsg := pb.RawMsg{
		ID:     int32(mvba.ID),
		Round:  int32(loop_info.round),
		Type:   2,
		Values: PBC1msgHash,
		Loop:   int32(loop_info.loop),
	}
	var PBC2_signs []pb.Signature
	for {
		var PBC2msg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_mine_PBC")
			return
		default:
			select {
			case PBC2msg = <-loop_info.PBC2_CH:
			case <-loop_info.PBC_done:
				return
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_mine_PBC")
				return
			}
		}
		if PBC2msg.RawMsg.Loop < int32(loop_info.loop) {
			fmt.Println("get a old PBC2 msg from", PBC2msg.RawMsg.ID)
			continue
		} else if PBC2msg.RawMsg.Loop > int32(loop_info.loop) {
			fmt.Println("get a future PBC2 msg from", PBC2msg.RawMsg.ID)
			continue
		}
		//**11 fmt.Println("get a PBC2msg from: ", PBC2msg.Sign.ID, "in handle mine pbc")
		//check msg
		if !bytes.Equal(PBC1msgHash, PBC2msg.RawMsg.Values) {
			panic(fmt.Sprintln("wrong PBC2 msg of ", PBC2msg.Sign.ID))
			return
		}
		//check sign
		/*if !mvba.verifySign_RawMsg(PBC2_RawMsg, *PBC2msg.Sign) {
			panic(fmt.Sprintln("wrong signature for PBC2 for ", mvba.ID))
			return
		}*/
		PBC2_signs = append(PBC2_signs, *PBC2msg.Sign)
		//loop_info.sigmas[mvba.ID-1] = append(loop_info.sigmas[mvba.ID-1], PBC2msg.Sign)
		PBC2_count++
		if PBC2_count >= max {
			break
		}
	}

	//sign myself
	myPBC2_sign := mvba.signRaw(PBC2_RawMsg)
	//loop_info.sigmas[mvba.ID-1] = append(loop_info.sigmas[mvba.ID-1], &myPBC2_sign)
	PBC2_signs = append(PBC2_signs, myPBC2_sign)
	//working 2/2: batch PBC2 signatures

	PBC3batchsign := mvba.batchSignRaw(PBC2_RawMsg, PBC2_signs)
	ret := mvba.verifySigns_RawMsg(PBC2_RawMsg, PBC3batchsign)
	if !ret {
		panic("wrong batch signature")
	}

	loop_info.sigmas[mvba.ID-1] = PBC3batchsign
	//generate PBC3 msg
	PBC3msg := pb.Msg{
		RawMsg: &pb.RawMsg{
			ID:     int32(mvba.ID),
			Round:  int32(loop_info.round),
			Type:   3,
			Values: PBC1msgHash,
			Loop:   int32(loop_info.loop),
		},
		SS: &PBC3batchsign,
	}
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle mine PBC")
		return
	default:
		select {
		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			return
		default:
			//**11 fmt.Println("broadcast PBC3 msg in handle mine PBC")
			mvba.broadcast(PBC3msg)
		}
	}
	//wait PBC4 msg
	PBC4_count := 0
	PBC4_RawMsg := pb.RawMsg{
		ID:     int32(mvba.ID),
		Round:  int32(loop_info.round),
		Type:   4,
		Values: PBC1msgHash,
		Loop:   int32(loop_info.loop),
	}
	var PBC4_signs []pb.Signature
	for {
		var PBC4msg pb.Msg
		select {
		case PBC4msg = <-loop_info.PBC4_CH:

		case <-loop_info.PBC_done:
			return
		case <-mvba.MVBA_done:
			return
		}

		if PBC4msg.RawMsg.Loop < int32(loop_info.loop) {
			fmt.Println("get a future PBC4 msg from", PBC4msg.RawMsg.ID)
			continue
		} else if PBC4msg.RawMsg.Loop > int32(loop_info.loop) {
			fmt.Println("get a future PBC4 msg from", PBC4msg.RawMsg.ID)
			continue
		}
		//**11 fmt.Println("geat a PBC4msg from:", PBC4msg.Sign.ID)
		//check msg
		if !bytes.Equal(PBC1msgHash, PBC4msg.RawMsg.Values) {
			panic("wrong PBC2 msg")
			return
		}
		//check sign
		/*if !mvba.verifySign_RawMsg(PBC4_RawMsg, *PBC4msg.Sign) {
			panic("wrong signature for PBC4")
			return
		}*/
		PBC4_signs = append(PBC4_signs, *PBC4msg.Sign)
		//loop_info.pis[mvba.ID-1] = append(loop_info.pis[mvba.ID-1], PBC4msg.Sign)

		PBC4_count++
		if PBC4_count >= max {
			break
		}
	}
	//sign myself
	myPBC4_sign := mvba.signRaw(PBC4_RawMsg)
	PBC4_signs = append(PBC4_signs, myPBC4_sign)
	Pis_batchsign := mvba.batchSignRaw(PBC4_RawMsg, PBC4_signs)

	ret = mvba.verifySigns_RawMsg(PBC4_RawMsg, Pis_batchsign)
	if !ret {
		panic("wrong batch signature")
	}

	//generate pi and ok
	ok := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "ok"))
	ok_sign := mvba.sign(ok)

	loop_info.Pi_Count_Lock.Lock()
	loop_info.OK = append(loop_info.OK, ok_sign)
	loop_info.Pi_count++
	if loop_info.Pi_count > max {
		ok_batch := mvba.batchSign(ok, loop_info.OK)
		ok := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "ok"))
		ret := mvba.verifySigns(ok, ok_batch)
		if !ret {
			panic("wrong batch signature")
		}
		//loop_info.OK_batch = ok_batch
		loop_info.Share_ready <- ok_batch
		SafeClose(loop_info.PBC_done)
		//**11 fmt.Println("close PBC_done in handle_mine_PBC")
	}
	loop_info.Pi_Count_Lock.Unlock()
	Pismsg := pb.Msg{
		RawMsg: &pb.RawMsg{
			ID:     int32(mvba.ID),
			Round:  int32(loop_info.round),
			Type:   5,
			Values: loop_info.inputs[mvba.ID-1],
			Loop:   int32(loop_info.loop),
		},
		Sign: &ok_sign,
		SS:   &Pis_batchsign,
	}
	select {
	case <-mvba.MVBA_done:
		return
	default:
		//**11 fmt.Println("broadcast pis msg in handle mine PBC")
		mvba.broadcast(Pismsg)
	}

}

func (mvba *MVBA) handle_pi(loop_info *loopInfo) {
	max := mvba.Node_num * 2 / 3

	for {
		var Pimsg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_pi")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_pi")
				return
			case Pimsg = <-loop_info.Pi_CH:
			}
		}
		if Pimsg.RawMsg.Loop < int32(loop_info.loop) {
			continue
		} else if Pimsg.RawMsg.Loop > int32(loop_info.loop) {
			fmt.Println("get a future loop pi msg from", Pimsg.RawMsg.ID)
			continue
		}
		//**11 fmt.Println("get a msg of pi in handle_pi from:", Pimsg.RawMsg.ID)
		//check msg
		Pi_RawMsg := pb.RawMsg{
			ID:     int32(Pimsg.RawMsg.ID),
			Round:  int32(loop_info.round),
			Type:   4,
			Values: mvba.hash(Pimsg.RawMsg.Values),
			Loop:   int32(loop_info.loop),
		}

		if !mvba.verifySigns_RawMsg(Pi_RawMsg, *Pimsg.SS) {
			panic("wrong pis signs for pi")
			return
		}

		ok := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "ok"))
		/*if !mvba.verifySign(ok, *Pimsg.Sign) {
			panic("wrong ok sign for pi")
			return
		}*/
		loop_info.inputs[Pimsg.RawMsg.ID-1] = Pimsg.RawMsg.Values
		loop_info.pis[Pimsg.RawMsg.ID-1] = *Pimsg.SS

		loop_info.Pi_Count_Lock.Lock()
		loop_info.OK = append(loop_info.OK, *Pimsg.Sign)
		loop_info.Pi_count++

		if loop_info.Pi_count > max {
			//working 2/2 : batch oksign

			ok_batch := mvba.batchSign(ok, loop_info.OK)
			ret := mvba.verifySigns(ok, ok_batch)
			if !ret {
				panic("wrong batch signature")
			}
			//loop_info.OK_batch = ok_batch
			//**11
			fmt.Println("len of oks:", len(loop_info.OK), "len of batch:", len(ok_batch.Mems), "len of batch:", len(loop_info.OK_batch.Mems))
			loop_info.Share_ready <- ok_batch
			SafeClose(loop_info.PBC_done)
			//**11 fmt.Println("close PBC_done in handle_pi")
			loop_info.Pi_Count_Lock.Unlock()
			break
		}
		loop_info.Pi_Count_Lock.Unlock()

	}
}

func (mvba *MVBA) send_share(loop_info *loopInfo) {
	var batchsign pb.BatchSignature
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return send_share")
		return
	default:
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return send_share")
			return
		case batchsign = <-loop_info.Share_ready:
		}
	}

	Share_RawMsg := pb.RawMsg{
		ID:    int32(mvba.ID),
		Round: int32(loop_info.round),
		Type:  6,
		Loop:  int32(loop_info.loop),
	}
	sharebyte := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "share"))
	share := mvba.hash(sharebyte)
	share_sign := mvba.sign(share)
	Sharemsg := pb.Msg{
		RawMsg: &Share_RawMsg,
		Sign:   &share_sign,
		SS:     &batchsign,
	}
	//**11 fmt.Println("2:len of oks:", len(loop_info.OK), "len of batch:", len(batchsign.Mems))
	//**11 fmt.Println("check share")
	/*Sharemsgbyte, err := proto.Marshal(&Sharemsg)
	if err != nil {
		panic(err)
	}
	tmpshare := pb.Msg{}
	err = proto.Unmarshal(Sharemsgbyte, &tmpshare)
	if err != nil {
		fmt.Println(Sharemsg)
		fmt.Println(Sharemsgbyte)
		panic(err)
	}*/
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return send_share")
		return
	default:
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return send_share")
			return
		default:
			//**11 fmt.Println("broadcast share in send_share")
			loop_info.Share_CH_mine <- Sharemsg
			mvba.broadcast(Sharemsg)
		}
	}

}

//shares are used to elect leader
func (mvba *MVBA) handle_share(loop_info *loopInfo) {
	max := mvba.Node_num * 2 / 3
	share_count := 0
	share_ready := true

	ok := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "ok"))
	sharebyte := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "share"))
	share := mvba.hash(sharebyte)
	for {
		var sharemsg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_share")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_share")
				return
			case sharemsg = <-loop_info.Share_CH:
				//**11 fmt.Println("get a share msg from:", sharemsg.RawMsg.ID)
				//check msg
				//check OK
				if !mvba.verifySigns(ok, *sharemsg.SS) {
					//11
					panic(fmt.Sprintln("wrong OK signs for share of loop ", sharemsg.RawMsg.Loop))
					return
				}
				//check share
				/*if !mvba.verifySign(share, *sharemsg.Sign) {
					//11
					panic("wrong share sign for share")
					return
				}*/
			case sharemsg = <-loop_info.Share_CH_mine:

			}
		}
		if sharemsg.RawMsg.Loop < int32(loop_info.loop) {
			continue
		} else if sharemsg.RawMsg.Loop > int32(loop_info.loop) {
			fmt.Println("get a future share msg from ", sharemsg.RawMsg.ID)
			continue
		}

		share_count++
		//loop_info.Pi_Count_Lock.Lock()
		loop_info.Shares = append(loop_info.Shares, *sharemsg.Sign)
		if share_ready {
			//loop_info.OK_batch = *sharemsg.SS
			loop_info.Share_ready <- *sharemsg.SS
			SafeClose(loop_info.PBC_done)
			//**11 fmt.Println("close PBC_done in handle_share")
			share_ready = false
		}
		//loop_info.Pi_Count_Lock.Unlock()
		if share_count > max {
			//batch shares
			loop_info.Share_batch = mvba.batchSign(share, loop_info.Shares)
			ret := mvba.verifySigns(share, loop_info.Share_batch)
			if !ret {
				if !ret {
					panic("wrong batch signature")
				}
			}

			//fake elect
			var bigNum big.Int
			bigNum.SetBytes(share)
			var bigNum2 big.Int
			bigNum2.SetInt64(int64(mvba.Node_num))
			var bigNum3 big.Int
			bigNum3.Set(&bigNum)
			bigNum3.Mod(&bigNum3, &bigNum2)
			j := bigNum3.Int64()
			//to be done: store coin in mvba
			//**11
			fmt.Println("get the coin ", j)
			loop_info.coin_ready <- int(j)
			if len(mvba.OldCoins) <= loop_info.loop {
				mvba.OldCoins = append(mvba.OldCoins, int(j))
			}

			//generate prevote msg

			return
		}

	}

}

func (mvba *MVBA) handle_finish(loop_info *loopInfo) {
	prevoteno_msg := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "prevno"))
	voteno_msg := []byte(fmt.Sprintf("%d%d%d%s", loop_info.round, loop_info.loop, loop_info.MVBAround, "voteno"))

	//**11
	fmt.Println("inside handle_finish")
	coin := 0
	max := mvba.Node_num * 2 / 3
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_finish")
		return
	default:
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		case coin = <-loop_info.coin_ready:
		}
	}
	//check if have received corresponding pis, and judge if should halt
	if len(loop_info.pis[coin].Mems) >= max {
		//generate halt msg
		Haltmsg := pb.Msg{
			RawMsg: &pb.RawMsg{
				ID:     int32(coin + 1),
				Round:  int32(loop_info.round),
				Type:   11,
				Values: loop_info.inputs[coin],
				Loop:   int32(loop_info.loop),
			},
			SS:  &loop_info.pis[coin],
			SS2: &loop_info.Share_batch,
		}
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_finish")
				return
			default:
				SafeClose(mvba.MVBA_done)
				//**11 fmt.Println("close mvba done")
			}
		}
		mvba.broadcast(Haltmsg)
		//**11 fmt.Println("broadcast halt msg in handle finish 1")
		mvba.outputCH <- Haltmsg
		//**11 fmt.Println("done send output")
		return
	}

	//check if have received corresponding sigmas, and judge what kind of prevote msg to send
	yes_flag := false
	var no_combins []pb.Signature
	var Prevmsg pb.Msg
	if len(loop_info.sigmas[coin].Mems) >= max {
		//generate prev yes msg and send it
		Prevmsg = pb.Msg{
			RawMsg: &pb.RawMsg{
				ID:     int32(mvba.ID),
				Round:  int32(loop_info.round),
				Type:   7,
				Values: loop_info.inputs[coin],
				Loop:   int32(loop_info.loop),
			},
			SS:       &loop_info.sigmas[coin],
			OldProof: mvba.OldProofs,
		}
		//**11 fmt.Println("3: len of sigmas:", len(loop_info.sigmas[coin].Mems))

		yes_flag = true

	} else {

		//generate prev no msg and send it
		//**11
		fmt.Println("generate prev no msg")
		no_sign := mvba.sign(prevoteno_msg)
		Prevmsg = pb.Msg{
			RawMsg: &pb.RawMsg{
				ID:    int32(mvba.ID),
				Round: int32(loop_info.round),
				Type:  8,
				Loop:  int32(loop_info.loop),
			},
			Sign:     &no_sign,
			OldProof: mvba.OldProofs,
		}

		no_combins = append(no_combins, no_sign)

	}
	select {
	case <-mvba.MVBA_done:
		//**11 fmt.Println("return handle_finish")
		return
	default:
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
		}
	}
	//**11 fmt.Println("broadcast prev msg in handle finish")
	mvba.broadcast(Prevmsg)

	//wait other prev msg

	prev_count := 0

	for {
		var prevmsg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_finish")
				return
			case prevmsg = <-loop_info.Prev_CH:
			}
		}
		if prevmsg.RawMsg.Loop != int32(loop_info.loop) {
			continue
		}
		//**11 fmt.Println("get a prev msg from:", prevmsg.RawMsg.ID)
		//check msg
		if prevmsg.RawMsg.Type == 7 {
			//**11 fmt.Println("get a prev msg of yes")
			//handle prev yes msg
			//check sigmas
			if yes_flag {
				prev_count++
			} else {
				PBC2_RawMsg := pb.RawMsg{
					ID:     int32(coin + 1),
					Round:  int32(loop_info.round),
					Type:   2,
					Values: mvba.hash(prevmsg.RawMsg.Values),
					Loop:   int32(loop_info.loop),
				}

				if mvba.verifySigns_RawMsg(PBC2_RawMsg, *prevmsg.SS) {
					yes_flag = true
					prev_count++
					//update loop_info.input
					/*res := mvba.check_input(prevmsg.RawMsg.Values, mvba.lastCommit, mvba.Node_num*mvba.K, mvba.sigmeta)
					if !res {
						//11
						fmt.Println("wrong input in prev")
						return
					}*/
					//check old proof
					/*if loop_info.round > 0 {
								if !mvba.check_values(&prevmsg) {
									//11
					fmt.Println("wrong proof for input")
									return
								}
							}*/
					loop_info.inputs[coin] = prevmsg.RawMsg.Values
					loop_info.sigmas[coin] = *prevmsg.SS

				}

			}

		} else if prevmsg.RawMsg.Type == 8 {
			//handle prev no msg

			//**11fmt.Println("get a prev msg of no")
			if yes_flag {
				prev_count++
			} else {
				/*if mvba.verifySign(prevoteno_msg, *prevmsg.Sign) {
					no_combins = append(no_combins, *prevmsg.Sign)
					prev_count++
				} else {
					//11
					panic("wrong prevno sign for prev msg")
					return
				}*/
				no_combins = append(no_combins, *prevmsg.Sign)
				prev_count++
			}

		}
		if prev_count >= max {
			break
		}

	}

	vote_count := 0
	vote_yes_count := 0
	vote_no_count := 0
	var vote_yes_combine []pb.Signature
	var vote_no_combine []pb.Signature
	//generate vote msg
	if yes_flag {
		//vote yes
		//generate pi
		PBC4_RawMsg := pb.RawMsg{
			ID:     int32(coin + 1),
			Round:  int32(loop_info.round),
			Type:   4,
			Values: mvba.hash(loop_info.inputs[coin]),
			Loop:   int32(loop_info.loop),
		}
		PBC4_Sign := mvba.signRaw(PBC4_RawMsg)

		Votemsg := pb.Msg{
			RawMsg: &pb.RawMsg{
				ID:     int32(coin + 1),
				Round:  int32(loop_info.round),
				Type:   9,
				Values: loop_info.inputs[coin],
				Loop:   int32(loop_info.loop),
			},
			Sign: &PBC4_Sign,
			SS:   &loop_info.sigmas[coin],
		}
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_finish")
				return
			default:
			}
		}
		//**11 fmt.Println("broadcast vote yes msg in handle finish")
		//**11 fmt.Println("len of vote sigmas:", len(loop_info.sigmas[coin].Mems))
		mvba.broadcast(Votemsg)
		vote_count++
		vote_yes_count++
		vote_yes_combine = append(vote_yes_combine, PBC4_Sign)
	} else {
		//vote no
		//batch prevote no signs
		prevoteno_batch := mvba.batchSign(prevoteno_msg, no_combins)
		ret := mvba.verifySigns(prevoteno_msg, prevoteno_batch)
		if !ret {
			panic("wrong batch signature")
		}

		Voteno_sign := mvba.sign(voteno_msg)
		Votemsg := pb.Msg{
			RawMsg: &pb.RawMsg{
				ID:    int32(coin + 1),
				Round: int32(loop_info.round),
				Type:  10,
				Loop:  int32(loop_info.loop),
			},
			Sign: &Voteno_sign,
			SS:   &prevoteno_batch,
		}
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_finish")
				return
			default:
			}
		}
		//**11 fmt.Println("broadcast vote 0 msg in handle finish")
		mvba.broadcast(Votemsg)
		vote_count++
		vote_no_count++
		vote_no_combine = append(vote_no_combine, Voteno_sign)
	}

	//wait vote msg
	for {
		var Votemsg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_finish")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_finish")
				return
			case Votemsg = <-loop_info.Vote_CH:
			}
		}
		if Votemsg.RawMsg.Loop != int32(loop_info.loop) {
			continue
		}
		if Votemsg.RawMsg.Round != int32(loop_info.MVBAround) {
			continue
		}
		//**11 fmt.Println("get a vote msg from:", Votemsg.Sign.ID)
		//check msg
		if Votemsg.RawMsg.Type == 9 {
			//handle vote yes
			PBC2_RawMsg := pb.RawMsg{
				ID:     int32(coin + 1),
				Round:  int32(loop_info.round),
				Type:   2,
				Values: mvba.hash(Votemsg.RawMsg.Values),
				Loop:   int32(loop_info.loop),
			}
			if !mvba.verifySigns_RawMsg(PBC2_RawMsg, *Votemsg.SS) {
				panic("wrong sigmas for vote yes")
				return
			}
			loop_info.sigmas[coin] = *Votemsg.SS

			/*PBC4_RawMsg := pb.RawMsg{
				ID:     int32(coin + 1),
				Round:  int32(loop_info.round),
				Type:   4,
				Values: mvba.hash(Votemsg.RawMsg.Values),
				Loop:   int32(loop_info.loop),
			}

			if !mvba.verifySign_RawMsg(PBC4_RawMsg, *Votemsg.Sign) {
				panic("wrong pi for vote yes")
				return
			}*/

			if len(loop_info.inputs[coin]) == 0 {
				/*res := mvba.check_input(Votemsg.RawMsg.Values, mvba.lastCommit, mvba.Node_num*mvba.K, mvba.sigmeta)
				if !res {
					//11
					fmt.Println("wrong input vote")
					return
				}*/

				//check old proof
				/*if loop_info.round > 0 {
					if !mvba.check_values(&Votemsg) {
						fmt.Println("wrong proof for input")
						return
					}
				}*/
				loop_info.inputs[coin] = Votemsg.RawMsg.Values
			}

			vote_count++
			vote_yes_count++
			vote_yes_combine = append(vote_yes_combine, *Votemsg.Sign)

		} else if Votemsg.RawMsg.Type == 10 {
			//handle vote no
			if !mvba.verifySigns(prevoteno_msg, *Votemsg.SS) {
				panic("wrong prev signs for vote no msg")
				return
			}
			if !mvba.verifySign(voteno_msg, *Votemsg.Sign) {
				panic("wrong vote no sign for vote no msg")
				return
			}
			vote_count++
			vote_no_count++
			//vote_no_combine = append(vote_yes_combine, *Votemsg.Sign)
			vote_no_combine = append(vote_no_combine, *Votemsg.Sign)

		}

		if vote_count > max {
			if vote_yes_count > max {
				//halt and output
				//if get pis before, use it; otherwise use vote_yes_combine
				var Haltmsg pb.Msg
				if len(loop_info.pis[coin].Mems) > 0 {
					Haltmsg = pb.Msg{
						RawMsg: &pb.RawMsg{
							ID:     int32(coin + 1),
							Round:  int32(loop_info.round),
							Type:   11,
							Values: loop_info.inputs[coin],
							Loop:   int32(loop_info.loop),
						},
						SS:  &loop_info.pis[coin],
						SS2: &loop_info.Share_batch,
					}
				} else {
					//batch vote yes signs
					PBC4_RawMsg := pb.RawMsg{
						ID:     int32(coin + 1),
						Round:  int32(loop_info.round),
						Type:   4,
						Values: mvba.hash(Votemsg.RawMsg.Values),
						Loop:   int32(loop_info.loop),
					}
					voteyes_batch := mvba.batchSignRaw(PBC4_RawMsg, vote_yes_combine)
					ret := mvba.verifySigns_RawMsg(PBC4_RawMsg, voteyes_batch)
					if !ret {
						panic("wrong batch signature")
					}

					Haltmsg = pb.Msg{
						RawMsg: &pb.RawMsg{
							ID:     int32(coin + 1),
							Round:  int32(loop_info.round),
							Type:   11,
							Values: loop_info.inputs[coin],
							Loop:   int32(loop_info.loop),
						},
						SS:  &voteyes_batch,
						SS2: &loop_info.Share_batch,
					}
				}
				select {
				case <-mvba.MVBA_done:
					//11
					fmt.Println("return handle_finish")
					return
				default:
					select {
					case <-mvba.MVBA_done:
						//11
						fmt.Println("return handle_finish")
						return
					default:
						SafeClose(mvba.MVBA_done)
						//11
						fmt.Println("close mvba done")
					}
				}
				//**11 fmt.Println("broadcast halt msg in handle finish 2")
				mvba.broadcast(Haltmsg)
				mvba.outputCH <- Haltmsg
				return
			} else if vote_no_count > max {
				//generate input of next loop using own input
				//batch vote no signs
				voteno_batch := mvba.batchSign(voteno_msg, vote_no_combine)
				ret := mvba.verifySigns(voteno_msg, voteno_batch)
				if !ret {
					panic("wrong batch signature")
				}

				mvba.OldProofs = append(mvba.OldProofs, &pb.Proof{Type: false, BatchSigns: &voteno_batch})
				Iputmsg := pb.Msg{
					RawMsg: &pb.RawMsg{
						ID:    int32(mvba.ID),
						Round: int32(loop_info.round),
						Type:  1,
						Loop:  int32(loop_info.loop + 1),
					},
					OldProof: mvba.OldProofs,
				}
				//mvba.loopinputCH <- Iputmsg
				mvba.inputlater <- Iputmsg
				//**11 fmt.Println("broadcast input in vote no")
				select {
				case <-mvba.MVBA_done:
					//11
					fmt.Println("return handle_finish")
					return
				default:
					select {
					case <-mvba.MVBA_done:
						//11
						fmt.Println("return handle_finish")
						return
					default:
					}
				}
				mvba.Loop++
				//loop_info.lastinput = loop_info.inputs[mvba.ID-1]

			} else {
				//generate input of next loop using coin input
				/*tmpproof := make([]*pb.Proof, 1)
				tmpproof[0] = &pb.Proof{Type: true, BatchSigns: &loop_info.sigmas[coin]}
				mvba.OldProofs = tmpproof*/
				mvba.OldProofs = append(mvba.OldProofs[0:0], &pb.Proof{Type: true, BatchSigns: &loop_info.sigmas[coin]})
				Iputmsg := pb.Msg{
					RawMsg: &pb.RawMsg{
						ID:     int32(mvba.ID),
						Round:  int32(loop_info.round),
						Type:   1,
						Values: loop_info.inputs[coin],
						Loop:   int32(loop_info.loop + 1),
					},
					OldProof: mvba.OldProofs,
				}
				//mvba.loopinputCH <- Iputmsg

				mvba.inputlater <- Iputmsg
				//**11 fmt.Println("broadcast input in vote yes and no")
				select {
				case <-mvba.MVBA_done:
					//11
					fmt.Println("return handle_finish")
					return
				default:
					select {
					case <-mvba.MVBA_done:
						//11
						fmt.Println("return handle_finish")
						return
					default:
					}
				}
				mvba.Loop++
				//loop_info.lastinput = loop_info.inputs[coin]

			}
			break
		}
	}

}

func (mvba *MVBA) handle_halt(Roundmvba int, MVBARoud int) {
	for {
		var Haltmsg pb.Msg
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_halt")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_halt")
				return
			case Haltmsg = <-mvba.haltCH:
			}
		}
		//**11 fmt.Println("get a halt mag")
		if Haltmsg.RawMsg.Round < int32(Roundmvba) {
			continue
		}

		//check share
		sharebyte := []byte(fmt.Sprintf("%d%d%d%s", int(Haltmsg.RawMsg.Round), int(Haltmsg.RawMsg.Loop), MVBARoud, "share"))
		share := mvba.hash(sharebyte)
		if !mvba.verifySigns(share, *Haltmsg.SS2) {
			panic(fmt.Sprintln("wrong share sign for halt msg of round", Haltmsg.RawMsg.Round, Roundmvba, MVBARoud))
			continue
		}
		var bigNum big.Int
		bigNum.SetBytes(share)
		var bigNum2 big.Int
		bigNum2.SetInt64(int64(mvba.Node_num))
		var bigNum3 big.Int
		bigNum3.Set(&bigNum)
		bigNum3.Mod(&bigNum3, &bigNum2)
		j := bigNum3.Int64()
		//**11 fmt.Println("get the coin ", j, " in halt")
		coin := int(j)
		if len(mvba.OldCoins) <= mvba.Loop {
			mvba.OldCoins = append(mvba.OldCoins, int(j))
		}

		if coin+1 != int(Haltmsg.RawMsg.ID) {
			panic("wrong coin for halt msg")
			continue
		}

		PBC4_RawMsg := pb.RawMsg{
			ID:     Haltmsg.RawMsg.ID,
			Round:  int32(Roundmvba),
			Type:   4,
			Values: mvba.hash(Haltmsg.RawMsg.Values),
			Loop:   Haltmsg.RawMsg.Loop,
		}
		if !mvba.verifySigns_RawMsg(PBC4_RawMsg, *Haltmsg.SS) {
			panic("wrong pi sign for halt msg")
			continue
		}
		select {
		case <-mvba.MVBA_done:
			//**11 fmt.Println("return handle_halt")
			return
		default:
			select {
			case <-mvba.MVBA_done:
				//**11 fmt.Println("return handle_halt")
				return
			default:
				SafeClose(mvba.MVBA_done)
				//**11 fmt.Println("close mvba done")
			}
		}
		//**11 fmt.Println("broadcast halt in handle halt")
		mvba.broadcast(Haltmsg)
		mvba.outputCH <- Haltmsg
		return
	}
}
