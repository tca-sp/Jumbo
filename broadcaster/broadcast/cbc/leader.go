package broadcast

import (
	"bytes"
	"crypto/sha256"
	cy "dumbo_fabric/crypto/signature"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"fmt"
	"time"

	"github.com/golang/protobuf/proto"
)

var txbuffsize int = 20000

func NewBroadcast_leader(nid int, lid int, sid int, num int, sigmeta cy.Signature, input chan []byte, output chan []byte, msgIn chan []byte, msgOut chan SendMsg, db *leveldb.DB, testmode bool, batchsize int, signal2tp chan []byte) BC_l {

	newBC_l := BC_l{
		nid:     nid,
		lid:     lid,
		sid:     sid,
		num:     num,
		sigmeta: sigmeta,
		input:   input,
		output:  output,
		//txbuff:     make([][]byte, 0),
		msgIn:     msgIn,
		msgOut:    msgOut,
		height:    0,
		threshold: num * 2 / 3,
		//batchsize:  batchsize,
		db:          db,
		callhelpCH:  make(chan pb.CallHelp, 1000),
		testmode:    testmode,
		signal2tpCH: signal2tp,
	}
	return newBC_l

}

/*type inputs struct {
	txlen int
	txs   []byte
}*/

//messages router
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
				fmt.Println(bcl.nid, "get a illegal future msg")
			} else {
				//fmt.Println(bcl.nid, "get a lower msg")
			}
		case 3:
			callhelp := pb.CallHelp{}
			if err := proto.Unmarshal(bcmsg.Content, &callhelp); err != nil {
				panic(err)
			}
			if int(callhelp.Leader) == bcl.lid && int(callhelp.K) == bcl.sid && callhelp.Round <= int32(bcl.height) {
				bcl.callhelpCH <- callhelp
			}
		default:
			panic("get a wrong type msg")
		}

	}

}

func (bcl *BC_l) handle_callhelp() {
	//for a leader, it's impossible to receive a callhelp for future round msg, and it's sure that it can help others
	for {
		callhelp := <-bcl.callhelpCH
		bcblock, err := bcl.db.Get(IntToBytes(int(callhelp.Round)))
		if err != nil {
			panic(err)
		}

		//send bcblock back
		msg := pb.BCMsg{
			Type:    int32(4),
			Content: bcblock,
		}
		msgbyte, err := proto.Marshal(&msg)
		if err != nil {
			panic(err)
		}
		checkblock := pb.BCBlock{}
		err = proto.Unmarshal(bcblock, &checkblock)
		if err != nil {
			panic(err)
		}
		//fmt.Println(bcl.nid, "send a help msg to", callhelp.ID, "of round ", callhelp.Round, "and real round", checkblock.RawBC.Height)
		bcl.msgOut <- SendMsg{
			ID:      int(callhelp.ID),
			Type:    1,
			Content: msgbyte,
			Height:  int(callhelp.Round),
		}
	}

}

//var timestamp int64
//var txblk []byte
//var txcount int

func (bcl *BC_l) Start() {
	fmt.Println(bcl.nid, "start broadcast leader ", bcl.nid, bcl.lid, bcl.sid)

	//getinputflag := make(chan bool, 10)
	//inputCH := make(chan inputs, 10)
	//go bcl.handle_txin(getinputflag, inputCH)
	//isLastEmpty := true
	if !bcl.testmode {
		go bcl.handle_callhelp()
	}
	var timestamp int64
	isfirsttime := true
	for {
		paybackCH := make(chan pb.PayBack, 1000)
		close := make(chan bool)
		go bcl.handle_msgin(bcl.height, paybackCH, close)
		//isTimeout := false
		//get input

		//to be done: timeout, when timeout, send combine signatures without new tx
		/*var txblk []byte
		txblk = <-bcl.input*/

		//txblk, txcount = bcl.getinput(getinputflag, inputCH)
		var txblk []byte

		if isfirsttime {
			txblk = <-bcl.input
			isfirsttime = false
		} else {
			select {
			case txblk = <-bcl.input:
			default:
				bcl.signal2tpCH <- make([]byte, 1)
				fmt.Println("generate a signal")
				txblk = <-bcl.input
			}
		}

		timestamp = time.Now().UnixNano()

		txcount := len(txblk) / 250

		var rawBC pb.RawBC
		//fmt.Println(bcl.nid, "time in")
		//isLastEmpty = false
		//fmt.Println(bcl.nid, "get a input")
		hash := sha256.New()
		hash.Write(txblk)
		root := hash.Sum(nil)[:]
		//generate rawBC

		if bcl.height == 0 {
			rawBC = pb.RawBC{
				Height:    int32(bcl.height),
				Root:      root,
				Leader:    int32(bcl.nid),
				K:         int32(bcl.sid),
				Timestamp: timestamp,
				Txcount:   int32(txcount),
			}
		} else {
			rawBC = pb.RawBC{
				Lastblkid: bcl.lastblkID,
				Height:    int32(bcl.height),
				Root:      root,
				Leader:    int32(bcl.nid),
				K:         int32(bcl.sid),
				Timestamp: timestamp,
				Txcount:   int32(txcount),
			}
		}

		hash1 := sha256.New()
		hashMsg, _ := proto.Marshal(&rawBC)
		hash1.Write(hashMsg)
		blkID := hash1.Sum(nil)[:]

		signbyte := append(blkID, IntToBytes(int(rawBC.Height))...)
		signbyte = append(signbyte, IntToBytes(bcl.lid)...)
		signbyte = append(signbyte, IntToBytes(bcl.sid)...)
		//fmt.Println(bcl.nid,"sign byte:", signbyte)
		signcontent := bcl.sigmeta.Sign(signbyte)
		signature := pb.Signature{
			ID:      int32(bcl.nid),
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
			if i != bcl.nid {
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

		//check batchsign
		ret := bcl.sigmeta.BatchSignatureVerify(bcl.lastsigns, bcl.lastsignbyte, bcl.lastsignmems)
		if !ret {
			panic(fmt.Sprintln("check my batchsign ", ret))
		}

		//generate blk send back to bc_l_m
		backBlock := &pb.BCBlock{
			RawBC: block.RawBC,
			//Payload: block.Payload,
			Sign: block.Sign,
			BatchSigns: &pb.BatchSignature{
				Signs: bcl.lastsigns,
				Mems:  bcl.lastsignmems},
		}
		//fmt.Println(bcl.nid, "check if bcblock right ", len(block.Payload))
		backBlockByte, _ := proto.Marshal(backBlock)
		bcl.output <- backBlockByte
		//bcl.lastblock = block
		bcl.lastblkID = blkID
		if !bcl.testmode {
			bcl.db.Put(IntToBytes(bcl.height-1), blkbyte)
		}

		//fmt.Println(bcl.nid, "put block ", bcl.height-1, rawBC.Height, "into database")

		SafeClose(close)
	}

}

/*func (bcl *BC_l) handle_txin(flag chan bool, input chan inputs) {
	var txblk [][]byte
	var txcount int
	inputbuff := make(chan inputs, 20)
	for {
		select {
		case <-flag:
			//response getinput
			select {
			case inputbytes := <-inputbuff:
				input <- inputbytes
				break
			default:
				//don't have enough txs now
				if txcount > 0 {
					//batch the txs we have now
					txs := pb.TXs{
						Txs: txblk,
					}
					txsBytes, err := proto.Marshal(&txs)
					if err != nil {
						panic(err)
					}
					input <- inputs{txcount, txsBytes}
					txblk = txblk[0:0]
					txcount = 0

				} else {
					//wait for a tx and batch it
					tx := <-bcl.input
					var newtxblk [][]byte
					newtxblk = append(newtxblk, tx)
					txs := pb.TXs{
						Txs: newtxblk,
					}
					txsBytes, err := proto.Marshal(&txs)
					if err != nil {
						panic(err)
					}
					input <- inputs{1, txsBytes}
				}
			}

		default:
			select {
			case tx := <-bcl.input:
				//batch blk if get enough txs
				if txcount < bcl.batchsize {
					txblk = append(txblk, tx)
					txcount++
				}

				var txsBytes []byte
				var err error
				if txcount >= bcl.batchsize {
					txs := pb.TXs{
						Txs: txblk,
					}
					txsBytes, err = proto.Marshal(&txs)
					if err != nil {
						panic(err)
					}
				}
				select {
				case inputbuff <- inputs{txcount, txsBytes}:
					txblk = txblk[0:0]
					txcount = 0
				default:
					time.Sleep(time.Millisecond * 50)
				}
			default:
				time.Sleep(time.Millisecond * 50)
			}
		}

	}
}

func (bcl *BC_l) getinput(flag chan bool, inputCH chan inputs) ([]byte, int) {
	flag <- true
	input := <-inputCH
	return input.txs, input.txlen

}
*/
