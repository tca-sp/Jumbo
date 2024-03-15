package broadcast

import (
	"bytes"
	"crypto/sha256"
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_leader(id int, num int, sigmeta cy.Signature, input chan []byte, output chan pb.BCBlock, msgIn chan []byte, msgOut chan SendMsg, batchsize int) BC_l {

	newBC_l := BC_l{
		id:        id,
		num:       num,
		sigmeta:   sigmeta,
		input:     input,
		output:    output,
		msgIn:     msgIn,
		msgOut:    msgOut,
		height:    0,
		threshold: num * 2 / 3,
	}
	return newBC_l

}

func (bcl *BC_l) handle_msgin(height int, paybackCH chan pb.PayBack, close chan bool) {
	var bcmsg pb.BCMsg
	var bcmsgByte []byte
	for {

		select {
		case <-close:
			return
		case bcmsgByte = <-bcl.msgIn:
			//fmt.Println(bcl.nid,"payback len ", len(bcmsgByte))
			err := proto.Unmarshal(bcmsgByte, &bcmsg)
			if err != nil {
				panic(err)
			}
		}

		//map messages by type 1: BCBlock; 2: Payback; 3: CallHelp; 4: Help
		switch bcmsg.Type {
		case 2:
			payback := pb.PayBack{}
			err := proto.Unmarshal(bcmsg.Content, &payback)
			if err != nil {
				panic(err)
			}
			if payback.Round == int32(height) {
				paybackCH <- payback
			} else if payback.Round > int32(height) {
				fmt.Println(bcl.id, "get a illegal future msg")
			} else {
				//fmt.Println(bcl.nid, "get a lower msg")
			}
		default:
			panic("get a wrong type msg")
		}

	}

}

//var timestamp int64
//var txblk []byte
//var txcount int

func (bcl *BC_l) Start() {
	fmt.Println(bcl.id, "start broadcast leader ", bcl.id)

	var timestamp int64
	for {
		paybackCH := make(chan pb.PayBack, 1000)
		close := make(chan bool)
		go bcl.handle_msgin(bcl.height, paybackCH, close)

		var txblk []byte

		txblk = <-bcl.input
		timestamp = time.Now().UnixNano()
		txcount := len(txblk) / 253

		var rawBC pb.RawBC
		hash := sha256.New()
		hash.Write(txblk)
		root := hash.Sum(nil)[:]

		if bcl.height == 0 {
			rawBC = pb.RawBC{
				Height:    int32(bcl.height),
				Root:      root,
				Leader:    1,
				Timestamp: timestamp,
				Txcount:   int32(txcount),
			}
		} else {
			rawBC = pb.RawBC{
				Lastblkid: bcl.lastblkID,
				Height:    int32(bcl.height),
				Root:      root,
				Leader:    1,
				Timestamp: timestamp,
				Txcount:   int32(txcount),
			}
		}

		hash1 := sha256.New()
		hashMsg, _ := proto.Marshal(&rawBC)
		hash1.Write(hashMsg)
		blkID := hash1.Sum(nil)[:]

		signbyte := append(blkID, IntToBytes(int(rawBC.Height))...)
		//fmt.Println(bcl.nid,"sign byte:", signbyte)
		signcontent := bcl.sigmeta.Sign(signbyte)
		signature := pb.Signature{
			ID:      int32(bcl.id),
			Content: signcontent,
		}

		var block pb.BCBlock
		if bcl.height == 0 {
			block = pb.BCBlock{
				RawBC:   &rawBC,
				Payload: txblk,
				Sign:    &signature,
			}
		} else {
			lastbatchsigns := &pb.BatchSignature{
				Signs: bcl.lastsigns,
				Mems:  bcl.lastsignmems,
			}
			block = pb.BCBlock{
				RawBC:      &rawBC,
				Payload:    txblk,
				Sign:       &signature,
				BatchSigns: lastbatchsigns,
			}
		}
		blkbyte, err := proto.Marshal(&block)
		if err != nil {
			panic(err)
		}
		//broadcast msg
		msg := pb.BCMsg{
			Type:    1,
			Content: blkbyte,
		}
		msgbyte, err := proto.Marshal(&msg)
		if err != nil {
			panic(err)
		}
		//generate new bcblock and send out
		//fmt.Println(bcl.nid, "generate new bcblock of round", block.RawBC.Height, " and send out")
		for i := 1; i <= bcl.num; i++ {
			if i != bcl.id {
				//fmt.Println(bcl.nid, "send a bcblock to ", i)
				bcl.msgOut <- SendMsg{
					ID:      i,
					Type:    0,
					Content: msgbyte,
				}
			}
		}

		//wait for signs from followers
		bcl.lastsignbyte = signbyte

		var tmpfollowsigns []*pb.Signature
		signcount := 0
		for {
			payback := <-paybackCH
			//fmt.Println(bcl.nid, "get a payback")
			if bytes.Equal(payback.BlkID, blkID) {
				//res := bcl.sigmeta.Verify(int(payback.Sign.ID-1), payback.Sign.Content, signbyte)
				//if res {
				tmpfollowsigns = append(tmpfollowsigns, payback.Sign)
				signcount++
				if signcount >= bcl.threshold {
					bcl.height++
					break
				}

				//} else {
				//	panic(fmt.Sprintln(bcl.nid, "wrong sign"))
				//	continue
				//}

			} else {
				continue
			}

		}
		//batch signatures
		bcl.lastsignmems = make([]int32, len(tmpfollowsigns))
		tmpsigns := make([][]byte, len(tmpfollowsigns))
		for i, tmpsign := range tmpfollowsigns {
			bcl.lastsignmems[i] = tmpsign.ID
			tmpsigns[i] = tmpsign.Content
		}

		bcl.lastsigns = bcl.sigmeta.BatchSignature(tmpsigns, bcl.lastsignbyte, bcl.lastsignmems)

		ret := bcl.sigmeta.BatchSignatureVerify(bcl.lastsigns, bcl.lastsignbyte, bcl.lastsignmems)
		if !ret {
			panic(fmt.Sprintln("check my batchsign ", ret))
		}

		backBlock := &pb.BCBlock{
			RawBC: block.RawBC,
		}

		bcl.output <- *backBlock
		bcl.lastblkID = blkID

		SafeClose(close)
	}

}
