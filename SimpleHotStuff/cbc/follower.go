package broadcast

import (
	"bytes"
	"crypto/sha256"
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_follower(id int, num int, sigmeta cy.Signature, output chan pb.BCBlock, msgIn chan []byte, msgOut chan SendMsg) BC_f {

	newBC_f := BC_f{
		id:           id,
		num:          num,
		threshold:    num * 2 / 3,
		sigmeta:      sigmeta,
		futurebuffer: futurebuffer{-1, &sync.Mutex{}, make([]pb.BCBlock, 0)},
		output:       output,
		msgIn:        msgIn,
		msgOut:       msgOut,
		bcCH:         make(chan pb.BCMsg, 1000),
		height:       0,
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
		default:
			panic("get a wrong type msg")
		}

	}

}

func (bcf *BC_f) Start() {

	go bcf.handle_msgin()
	bcf.handle_bcblock()
}

//handle normal protocol msg, round can be added here
func (bcf *BC_f) handle_bcblock() {
	fmt.Println(bcf.id, "inside handle_bcblock")

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
			if !(int(bcblock.RawBC.Leader) == 1) {
				continue
			}

		}

		if bcblock.RawBC.Height < int32(bcf.height) {
		} else if bcblock.RawBC.Height == int32(bcf.height) {
			if isfuture {
				if !bcf.checkblockid(bcblock) {
					panic("get a wrong future block")
				} else {
					bcf.generatepayback(bcblock)

				}
			} else {
				bcf.check_bcblock(bcblock)
			}

		} else {
			if !bcf.checkleadersign(bcblock) {
				panic("get a wrong futureblock")
			}

			if !bcf.checkcomsigns(bcblock) {
				panic("get a wrong futureblock")
			}

			bcf.futurebuffer.put(bcblock)

		}

	}

}

func (bcf *BC_f) checkblockid(block pb.BCBlock) bool {
	if block.RawBC.Height != 0 {
		if !bytes.Equal(bcf.lastblkID, block.RawBC.Lastblkid) {
			panic(fmt.Sprintln(bcf.id, "get a wrong msg of height ", bcf.height))
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
			panic(fmt.Sprintln(bcf.id, "wrong combine signs of height ", bcf.height))
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
	res := bcf.sigmeta.Verify(int(bcf.id-1), block.Sign.Content, signbyte)
	if !res {
		panic(fmt.Sprintln(bcf.id, "wrong  sign of leader of height ", bcf.height, " of leader ", 1, " and ", block.Sign.ID))
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
		signcontent := bcf.sigmeta.Sign(signbyte)

		signature := pb.Signature{
			ID:      int32(bcf.id),
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
			ID:      1,
			Type:    0,
			Content: msgbyte,
		}

		//generate output
		if bcf.height > 0 {
			backBlock := &pb.BCBlock{
				RawBC: bcf.lastblock.RawBC,
			}

			//fmt.Println(bcf.nid,"sign id ", bcf.lastblock.Sign.ID)
			bcf.output <- *backBlock
		}
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
			panic(fmt.Sprintln(bcf.id, "get a wrong msg of height ", bcf.height))
			return
		} else {
			//check signs
			check := true
			signbyte := append(bcf.lastblkID, IntToBytes(int(block.RawBC.Height-1))...)
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
				panic(fmt.Sprintln(bcf.id, "wrong combine signs of height ", bcf.height))
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
	//fmt.Println(bcf.nid,"follower ", bcf.nid, " signbyte:", signbyte)
	res := bcf.sigmeta.Verify(0, block.Sign.Content, signbyte)
	//fmt.Println(bcf.nid,"follower ", bcf.nid, "sign content:", block.Sign.Content)
	if !res {
		panic(fmt.Sprintln(bcf.id, "wrong  sign of leader of height ", bcf.height, " of leader ", 1, " and ", block.Sign.ID))
		return
	}

	//generate payback
	//to be done: if data is empty, no need to send payback
	if block.RawBC.Root != nil {
		signbyte := append(blkID, IntToBytes(int(block.RawBC.Height))...)
		signcontent := bcf.sigmeta.Sign(signbyte)

		//check my signature
		/*ret := bcf.sigmeta.Verify(bcf.nid-1, signcontent, signbyte)
		fmt.Println(bcf.nid, "check my signature ", ret)*/

		signature := pb.Signature{
			ID:      int32(bcf.id),
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
			ID:      1,
			Type:    0,
			Content: msgbyte,
		}

		//generate output
		if bcf.height > 0 {
			backBlock := &pb.BCBlock{
				RawBC: bcf.lastblock.RawBC,
			}

			//fmt.Println(bcf.nid,"sign id ", bcf.lastblock.Sign.ID)
			bcf.output <- *backBlock
		}
	}

	//fmt.Println(bcf.nid, "put block ", bcf.height, block.RawBC.Height, "into database")
	//bcf.checkcallhelpbuf(bcf.height)
	bcf.lastblkID = blkID
	bcf.lastblock = block
	bcf.height++

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
