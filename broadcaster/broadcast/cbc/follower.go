package broadcast

import (
	"bytes"
	"crypto/sha256"
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"fmt"
	"math/rand"
	"sync"

	"github.com/golang/protobuf/proto"

	"dumbo_fabric/database/leveldb"

	mapset "github.com/deckarep/golang-set"
)

func NewBroadcast_follower(nid int, lid int, sid int, num int, sigmeta cy.Signature, output chan []byte, msgIn chan []byte, msgOut chan SendMsg, db *leveldb.DB, testmode bool) BC_f {

	newBC_f := BC_f{
		nid:            nid,
		lid:            lid,
		sid:            sid,
		num:            num,
		threshold:      num * 2 / 3,
		db:             db,
		sigmeta:        sigmeta,
		callhelp:       callhelp{false, -1, &sync.Mutex{}, mapset.NewSet[int]()},
		callhelpbuffer: callhelpbuffer{&sync.Mutex{}, make(map[int]mapset.Set[int])},
		futurebuffer:   futurebuffer{-1, &sync.Mutex{}, make([]pb.BCBlock, 0)},
		output:         output,
		msgIn:          msgIn,
		msgOut:         msgOut,
		bcCH:           make(chan pb.BCMsg, 1000),
		callhelpCH:     make(chan pb.CallHelp, 1000),
		helpCH:         make(chan pb.BCMsg, 1000),
		height:         0,
		testmode:       testmode,
	}
	return newBC_f

}

func (bcf *BC_f) handle_msgin() {

	for {
		var bcmsg pb.BCMsg
		var bcmsgByte []byte
		bcmsgByte = <-bcf.msgIn
		err := proto.Unmarshal(bcmsgByte, &bcmsg)
		if err != nil {
			panic(err)
		}

		//map messages by type 1: BCBlock; 2: Payback; 3: CallHelp; 4: Help
		switch bcmsg.Type {
		case 1:
			//get a propose from leader
			//fmt.Println(bcf.nid, "get a proposal")
			bcf.bcCH <- bcmsg

		case 3:
			//get a callhelp msg
			//fmt.Println(bcf.nid, "get a callhelp msg")
			callhelp := pb.CallHelp{}
			if err := proto.Unmarshal(bcmsg.Content, &callhelp); err != nil {
				panic(err)
			}
			if int(callhelp.Leader) == bcf.lid && int(callhelp.K) == bcf.sid {
				bcf.callhelpCH <- callhelp
			}
		case 4:
			//fmt.Println(bcf.nid, "get a help msg")
			bcf.bcCH <- bcmsg
		default:
			panic("get a wrong type msg")
		}

	}

}

func (bcf *BC_f) Start() {

	go bcf.handle_msgin()
	if !bcf.testmode {
		go bcf.handle_callhelp()
	}

	bcf.handle_bcblock()
}

//handle normal protocol msg, round can be added here
func (bcf *BC_f) handle_bcblock() {
	fmt.Println(bcf.nid, "inside handle_bcblock")

	for {
		isfuture := false
		bcblock := pb.BCBlock{}
		for {
			if bcf.futurebuffer.lowwest > 0 && bcf.futurebuffer.lowwest < bcf.height {
				bcf.futurebuffer.getlowwest()
			} else {
				break
			}
		}
		//first check futurebuffer whether contain right bcblock
		if bcf.futurebuffer.lowwest == bcf.height && bcf.height != 0 {
			bcblock, _ = bcf.futurebuffer.getlowwest()
			isfuture = true
		} else {
			bcmsg := <-bcf.bcCH
			//fmt.Println(bcf.nid, "handdle a fresh msg of type ", bcmsg.Type)
			/*if !bcf.callhelp.iscallhelp && bcmsg.Type == 4 {
				//if not callhelp, ignore help msg
				continue
			}*/
			if err := proto.Unmarshal(bcmsg.Content, &bcblock); err != nil {
				panic(err)
			}
			if !(int(bcblock.RawBC.Leader) == bcf.lid && int(bcblock.RawBC.K) == bcf.sid) {
				continue
			}

		}

		if bcblock.RawBC.Height < int32(bcf.height) {
			//fmt.Println(bcf.nid, "get an old msg")
			//old bcblock, abondon it directly
		} else if bcblock.RawBC.Height == int32(bcf.height) {
			//right bcblock
			//check bcblock
			//fmt.Println(bcf.nid, "get a right msg")
			if isfuture {
				//fmt.Println(bcf.nid, "get a right msg from buffer of round", bcblock.RawBC.Height)
				//from futurebuff, no need to check signs
				if !bcf.checkblockid(bcblock) {
					panic("get a wrong future block")
				} else {
					bcf.generatepayback(bcblock) //round added here
					//update missblocks
					//bcf.callhelp.remove(bcblock)

				}
			} else {
				//fmt.Println(bcf.nid, "get a right and fresh msg of round ", bcblock.RawBC.Height)
				bcf.check_bcblock(bcblock)
				//update missblocks
				//bcf.callhelp.remove(bcblock)
			}

		} else {
			//future bcblock, callhelp if it's a legal bcblock
			//check if legal
			//fmt.Println(bcf.nid, "get a future msg of round:", bcblock.RawBC.Height)
			if !bcf.checkleadersign(bcblock) {
				panic("get a wrong futureblock")
			}

			if !bcf.checkcomsigns(bcblock) {
				panic("get a wrong futureblock")
			}
			//callhelp

			if !bcf.testmode {
				go bcf.sendcallhelp(bcblock)
			} else {
				bcf.futurebuffer.put(bcblock)
			}

		}

	}

}

func (bcf *BC_f) sendcallhelp(block pb.BCBlock) {
	fmt.Println(bcf.nid, "inside callhelp")
	//add it to futurebuf
	bcf.futurebuffer.put(block)
	//caculate what blocks we should callhelp for
	begin, end := bcf.callhelp.put(block, bcf.height)
	if begin == end && begin == 0 {
		return
	}
	//send callhelp msg
	threshold := bcf.num/3*2 + 1
	for i := begin; i <= end; i++ {
		callhelp := pb.CallHelp{
			Round:  int32(i),
			ID:     int32(bcf.nid),
			Leader: int32(bcf.lid),
			K:      int32(bcf.sid),
		}
		callhelpbyte, err := proto.Marshal(&callhelp)
		if err != nil {
			panic(err)
		}
		msg := pb.BCMsg{
			Type:    3,
			Content: callhelpbyte,
		}
		msgbyte, err := proto.Marshal(&msg)
		if err != nil {
			panic(err)
		}

		randnum := rand.Int31n(int32(bcf.num)) + 1

		for j := 0; j < threshold; j++ {
			if ((int(randnum)+j)%bcf.num + 1) == bcf.nid {
				continue
			}
			//fmt.Println(bcf.nid, "send callhelp msg to ", (int(randnum)+j)%bcf.num+1, " of round ", i)
			bcf.msgOut <- SendMsg{
				ID:      (int(randnum)+j)%bcf.num + 1,
				Type:    2,
				Content: msgbyte,
			}
		}

	}
}

func (bcf *BC_f) checkblockid(block pb.BCBlock) bool {
	if block.RawBC.Height != 0 {
		if !bytes.Equal(bcf.lastblkID, block.RawBC.Lastblkid) {
			panic(fmt.Sprintln(bcf.nid, "get a wrong msg of height ", bcf.height))
			return false
		}
	}
	return true
}

func (bcf *BC_f) checkcomsigns(block pb.BCBlock) bool {
	//check blockid
	if block.RawBC.Height != 0 {

		//check signs
		check := true

		//check batch signatures
		signbyte := append(block.RawBC.Lastblkid, IntToBytes(int(block.RawBC.Height-1))...)
		signbyte = append(signbyte, IntToBytes(bcf.lid)...)
		signbyte = append(signbyte, IntToBytes(bcf.sid)...)
		check = bcf.sigmeta.BatchSignatureVerify(block.BatchSigns.Signs, signbyte, block.BatchSigns.Mems)
		/*for _, sign := range block.Signs {
			signbyte := append(block.RawBC.Lastblkid, IntToBytes(int(block.RawBC.Height-1))...)
			signbyte = append(signbyte, IntToBytes(bcf.lid)...)
			signbyte = append(signbyte, IntToBytes(bcf.sid)...)
			res := bcf.sigmeta.Verify(int(sign.ID-1), sign.Content, signbyte)
			if !res {
				check = false
				break
			}
		}*/
		if !check {
			panic(fmt.Sprintln(bcf.nid, "wrong combine signs of height ", bcf.height))
			return false
		}

	}
	return true

}

func (bcf *BC_f) checkleadersign(block pb.BCBlock) bool {
	hash := sha256.New()
	hashMsg, _ := proto.Marshal(block.RawBC)
	hash.Write(hashMsg)
	blkID := hash.Sum(nil)[:]
	signbyte := append(blkID, IntToBytes(int(block.RawBC.Height))...)
	signbyte = append(signbyte, IntToBytes(bcf.lid)...)
	signbyte = append(signbyte, IntToBytes(bcf.sid)...)
	res := bcf.sigmeta.Verify(int(bcf.lid-1), block.Sign.Content, signbyte)
	if !res {
		panic(fmt.Sprintln(bcf.nid, "wrong  sign of leader of height ", bcf.height, " of leader ", bcf.lid, " and ", block.Sign.ID))
		return false
	}
	return true
}

func (bcf *BC_f) generatepayback(block pb.BCBlock) {
	hash := sha256.New()
	hashMsg, _ := proto.Marshal(block.RawBC)
	hash.Write(hashMsg)
	blkID := hash.Sum(nil)[:]
	//generate payback
	//to be done: if data is empty, no need to send payback
	if block.RawBC.Root != nil {
		signbyte := append(blkID, IntToBytes(int(block.RawBC.Height))...)
		signbyte = append(signbyte, IntToBytes(bcf.lid)...)
		signbyte = append(signbyte, IntToBytes(bcf.sid)...)
		signcontent := bcf.sigmeta.Sign(signbyte)

		signature := pb.Signature{
			ID:      int32(bcf.nid),
			Content: signcontent,
		}
		payback := pb.PayBack{
			Round: int32(bcf.height),
			BlkID: blkID,
			Sign:  &signature,
		}
		paybackByte, err := proto.Marshal(&payback)
		if err != nil {
			panic(err)
		}
		msg := pb.BCMsg{
			Type:    2,
			Content: paybackByte,
		}
		msgbyte, err := proto.Marshal(&msg)
		if err != nil {
			panic(err)
		}
		//fmt.Println(bcf.nid, "send a payback")
		bcf.msgOut <- SendMsg{
			ID:      bcf.lid,
			Type:    0,
			Content: msgbyte,
		}

		//generate output
		if bcf.height > 0 {
			backBlock := &pb.BCBlock{
				RawBC: bcf.lastblock.RawBC,
				//Payload:    bcf.lastblock.Payload,
				Sign:       bcf.lastblock.Sign,
				BatchSigns: block.BatchSigns,
			}
			backBlock.RawBC.K = int32(bcf.sid)
			commitMsg, err := proto.Marshal(backBlock)
			if err != nil {
				panic(fmt.Sprintln(bcf.nid, err))
			}

			//fmt.Println(bcf.nid,"sign id ", bcf.lastblock.Sign.ID)
			bcf.output <- commitMsg
		}
	}
	bcbyte, err := proto.Marshal(&block)
	if err != nil {
		panic(err)
	}
	if !bcf.testmode {
		bcf.db.Put(IntToBytes(bcf.height), bcbyte)
	}

	//fmt.Println(bcf.nid, "put block ", bcf.height, block.RawBC.Height, "into database")
	//bcf.checkcallhelpbuf(bcf.height)

	bcf.lastblkID = blkID
	bcf.lastblock = block
	bcf.height++
}

func (bcf *BC_f) check_bcblock(block pb.BCBlock) {
	//fmt.Println(bcf.nid, "inside check_bcblock")
	//check blockid
	if block.RawBC.Height != 0 {
		if !bytes.Equal(bcf.lastblkID, block.RawBC.Lastblkid) {
			panic(fmt.Sprintln(bcf.nid, "get a wrong msg of height ", bcf.height))
			return
		} else {
			//check signs
			check := true
			signbyte := append(bcf.lastblkID, IntToBytes(int(block.RawBC.Height-1))...)
			signbyte = append(signbyte, IntToBytes(bcf.lid)...)
			signbyte = append(signbyte, IntToBytes(bcf.sid)...)
			//check batch signature
			check = bcf.sigmeta.BatchSignatureVerify(block.BatchSigns.Signs, signbyte, block.BatchSigns.Mems)
			/*for _, sign := range block.Signs {

				res := bcf.sigmeta.Verify(int(sign.ID-1), sign.Content, signbyte)
				if !res {
					check = false
					break
				}
			}*/
			if !check {
				panic(fmt.Sprintln(bcf.nid, "wrong combine signs of height ", bcf.height))
				return
			}
		}
	}

	//check sign of leader
	hash := sha256.New()
	hashMsg, _ := proto.Marshal(block.RawBC)
	hash.Write(hashMsg)
	blkID := hash.Sum(nil)[:]
	signbyte := append(blkID, IntToBytes(int(block.RawBC.Height))...)
	signbyte = append(signbyte, IntToBytes(bcf.lid)...)
	signbyte = append(signbyte, IntToBytes(bcf.sid)...)
	//fmt.Println(bcf.nid,"follower ", bcf.nid, " signbyte:", signbyte)
	res := bcf.sigmeta.Verify(int(bcf.lid-1), block.Sign.Content, signbyte)
	//fmt.Println(bcf.nid,"follower ", bcf.nid, "sign content:", block.Sign.Content)
	if !res {
		panic(fmt.Sprintln(bcf.nid, "wrong  sign of leader of height ", bcf.height, " of leader ", bcf.lid, " and ", block.Sign.ID))
		return
	}

	//generate payback
	//to be done: if data is empty, no need to send payback
	if block.RawBC.Root != nil {
		signbyte := append(blkID, IntToBytes(int(block.RawBC.Height))...)
		signbyte = append(signbyte, IntToBytes(bcf.lid)...)
		signbyte = append(signbyte, IntToBytes(bcf.sid)...)
		signcontent := bcf.sigmeta.Sign(signbyte)

		//check my signature
		/*ret := bcf.sigmeta.Verify(bcf.nid-1, signcontent, signbyte)
		fmt.Println(bcf.nid, "check my signature ", ret)*/

		signature := pb.Signature{
			ID:      int32(bcf.nid),
			Content: signcontent,
		}
		payback := pb.PayBack{
			Round: int32(bcf.height),
			BlkID: blkID,
			Sign:  &signature,
		}
		paybackByte, err := proto.Marshal(&payback)
		if err != nil {
			panic(err)
		}
		msg := pb.BCMsg{
			Type:    2,
			Content: paybackByte,
		}
		msgbyte, err := proto.Marshal(&msg)
		if err != nil {
			panic(err)
		}
		//fmt.Println(bcf.nid, "generate a payback")
		bcf.msgOut <- SendMsg{
			ID:      bcf.lid,
			Type:    0,
			Content: msgbyte,
		}

		//generate output
		if bcf.height > 0 {
			backBlock := &pb.BCBlock{
				RawBC: bcf.lastblock.RawBC,
				//Payload:    bcf.lastblock.Payload,
				Sign:       bcf.lastblock.Sign,
				BatchSigns: block.BatchSigns,
			}
			backBlock.RawBC.K = int32(bcf.sid)
			commitMsg, err := proto.Marshal(backBlock)
			if err != nil {
				panic(fmt.Sprintln(bcf.nid, err))
			}

			//fmt.Println(bcf.nid,"sign id ", bcf.lastblock.Sign.ID)
			bcf.output <- commitMsg
		}
	}
	bcbyte, err := proto.Marshal(&block)
	if err != nil {
		panic(err)
	}
	if !bcf.testmode {
		bcf.db.Put(IntToBytes(bcf.height), bcbyte)
	}

	//fmt.Println(bcf.nid, "put block ", bcf.height, block.RawBC.Height, "into database")
	//bcf.checkcallhelpbuf(bcf.height)
	bcf.lastblkID = blkID
	bcf.lastblock = block
	bcf.height++

}

//handle callhelp msg from others
func (bcf *BC_f) handle_callhelp() {

	for {
		callhelpmsg := <-bcf.callhelpCH
		//check if I can help now
		if callhelpmsg.Round < int32(bcf.height) {
			//callhelp for a block in db
			helpblock, err := bcf.db.Get(IntToBytes(int(callhelpmsg.Round)))
			if err != nil {
				panic(err)
			}
			helpmsg := pb.BCMsg{
				Type:    4,
				Content: helpblock,
			}
			helpmsgbyte, err := proto.Marshal(&helpmsg)
			if err != nil {
				panic(err)
			}
			//check help block
			checkblock := pb.BCBlock{}
			err = proto.Unmarshal(helpblock, &checkblock)
			if err != nil {
				panic(err)
			}
			//fmt.Println(bcf.nid, "send a help msg to", callhelpmsg.ID, "of round ", callhelpmsg.Round, "and real round", checkblock.RawBC.Height)
			bcf.msgOut <- SendMsg{
				ID:      int(callhelpmsg.ID),
				Type:    1,
				Content: helpmsgbyte,
				Height:  int(callhelpmsg.Round),
			}
		} else {
			//call for a future block, we can't help it, even it's in futurebuffer, as we can't 100% ensure the security of blocks in futurebuffer
			//buffer this call help msg, and check it whenever put a block in db
			//bcf.callhelpbuffer.put(int(callhelpmsg.Round), int(callhelpmsg.ID))

		}
	}
}

//check if can help others now
func (bcf *BC_f) checkcallhelpbuf(height int) {
	if height > 0 {
		//check height-1
		if value, ok := bcf.callhelpbuffer.buffer[height-1]; ok {
			for id := range value.Iter() {
				helpblock, err := bcf.db.Get(IntToBytes(height - 1))
				if err != nil {
					panic(err)
				}
				helpmsg := pb.BCMsg{
					Type:    4,
					Content: helpblock,
				}
				helpmsgbyte, err := proto.Marshal(&helpmsg)
				if err != nil {
					panic(err)
				}
				//fmt.Println(bcf.nid, "send a help msg in checkcallhelpbuf 1")
				bcf.msgOut <- SendMsg{
					ID:      id,
					Type:    1,
					Content: helpmsgbyte,
					Height:  height - 1,
				}
			}
			//to be done: free old buffer in callhelpbuffer
			delete(bcf.callhelpbuffer.buffer, height-1)
		}
	}
	//check height
	if value, ok := bcf.callhelpbuffer.buffer[height]; ok {
		for id := range value.Iter() {
			helpblock, err := bcf.db.Get(IntToBytes(height))
			if err != nil {
				panic(err)
			}
			helpmsg := pb.BCMsg{
				Type:    4,
				Content: helpblock,
			}
			helpmsgbyte, err := proto.Marshal(&helpmsg)
			if err != nil {
				panic(err)
			}
			//fmt.Println(bcf.nid, "send a help msg in checkcallhelpbuf 2")
			bcf.msgOut <- SendMsg{
				ID:      id,
				Type:    1,
				Content: helpmsgbyte,
				Height:  height,
			}
		}
		delete(bcf.callhelpbuffer.buffer, height)
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

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

//pop first block in buffer, don't call it when buffer is nil
func (fb *futurebuffer) getlowwest() (pb.BCBlock, int) {
	fb.lock.Lock()
	defer fb.lock.Unlock()

	var bcblock pb.BCBlock
	if len(fb.bcblocks) != 0 {
		bcblock = fb.bcblocks[0]
		fb.bcblocks = fb.bcblocks[1:]
		if len(fb.bcblocks) != 0 {
			fb.lowwest = int(fb.bcblocks[0].RawBC.Height)
		}
		//fmt.Println("inside getlowwest, set lowwest to", fb.lowwest)
		return bcblock, fb.lowwest
	} else {
		fb.lowwest = -1
		return pb.BCBlock{}, -1
	}

}

//put a block in buffer, and update lowwest
func (fb *futurebuffer) put(block pb.BCBlock) {
	fb.lock.Lock()
	defer fb.lock.Unlock()
	if len(fb.bcblocks) == 0 {
		fb.bcblocks = append(fb.bcblocks, block)
		fb.lowwest = int(block.RawBC.Height)
	} else {
		flag := false
		var tmp []pb.BCBlock
		for i := 0; i < len(fb.bcblocks); i++ {
			if block.RawBC.Height < fb.bcblocks[i].RawBC.Height {
				if !flag {
					tmp = append(tmp, block)
					flag = true
				}
				tmp = append(tmp, fb.bcblocks[i])
			} else if block.RawBC.Height == fb.bcblocks[i].RawBC.Height {
				tmp = append(tmp, fb.bcblocks[i])
				flag = true
			} else {
				tmp = append(tmp, fb.bcblocks[i])
				if i == len(fb.bcblocks)-1 {
					tmp = append(tmp, block)
				}
			}
		}
		fb.bcblocks = tmp
		if block.RawBC.Height < fb.bcblocks[0].RawBC.Height {
			fb.lowwest = int(block.RawBC.Height)
		}
	}
	//fmt.Println("set lowwest to", fb.lowwest)
}

//incoming a future bcblock, check if need to send new callhelp msg
//to be done: update highest
func (ch *callhelp) put(block pb.BCBlock, myheight int) (int, int) {
	ch.lock.Lock()
	//fmt.Println("caculate which block to callhelp ", block.RawBC.Height, myheight, ch.highest)
	defer ch.lock.Unlock()

	if int(block.RawBC.Height) > ch.highest {
		//need to send new callhelp msg
		/*for i := ch.highest + 1; i < int(block.RawBC.Height); i++ {
			ch.missblocks.Add(i)
		}*/
		if ch.highest+1 == int(block.RawBC.Height) {
			ch.highest = int(block.RawBC.Height)
			return 0, 0
		} else {
			var tmp int
			if ch.highest > myheight {
				tmp = ch.highest
			} else {
				tmp = myheight
			}

			ch.highest = int(block.RawBC.Height)
			return tmp, int(block.RawBC.Height) - 1
		}

	} else {
		/*if ch.missblocks.Contains(int(block.RawBC.Height)) {
			//remove this index
			ch.missblocks.Remove(int(block.RawBC.Height))

		}*/
		return 0, 0
	}

}

//after receive a legal bcblock or help msg, remove it from missblocks
func (ch *callhelp) remove(block pb.BCBlock) {
	ch.lock.Lock()
	defer ch.lock.Unlock()
	if ch.missblocks.Contains(int(block.RawBC.Height)) {
		ch.missblocks.Remove(int(block.RawBC.Height))
	}
}

//
func (chb *callhelpbuffer) put(key int, subvalue int) {
	chb.lock.Lock()
	defer chb.lock.Unlock()
	if value, ok := chb.buffer[key]; ok {
		//already buffer same round callhelp
		value.Add(subvalue)
	} else {
		chb.buffer[key] = mapset.NewSet[int]()
		chb.buffer[key].Add(subvalue)
	}

}
