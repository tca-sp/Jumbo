package wrbc

import (
	"bytes"
	"crypto/sha256"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"fmt"
	"log"
	"time"

	"github.com/golang/protobuf/proto"
)

var txbuffsize int = 10000

func NewBroadcast_leader(nid int, lid int, sid int, num int, input chan []byte, output chan []byte, msgIn chan []byte, msgOut chan SendMsg, log log.Logger, db leveldb.DB, testmode bool, batchsize int, signal2tp chan []byte) BC_l {
	newBC_l := BC_l{
		nid:       nid,
		lid:       lid,
		sid:       sid,
		num:       num,
		threshold: (num - 1) / 3,
		height:    0,
		//batchsize: batchsize,

		input:  input,
		output: output,
		//txbuff: make([][]byte, 0),
		msgIn:  msgIn,
		msgOut: msgOut,

		readyCH: make(chan pb.WRBCMsg, 400),
		echoCH:  make(chan pb.WRBCMsg, 400),

		callhelpCH: make(chan pb.WRBCMsg, 400),

		log:         log,
		db:          db,
		testmode:    testmode,
		signal2tpCH: signal2tp,
	}
	return newBC_l

}

//messages router
func (bcl *BC_l) handle_msgin() {
	bcl.log.Println("inside handle_msgin")
	var rbcmsg pb.WRBCMsg
	var rbcmsgByte []byte
	for {

		rbcmsgByte = <-bcl.msgIn
		err := proto.Unmarshal(rbcmsgByte, &rbcmsg)
		if err != nil {
			bcl.log.Fatalln(err)
		}
		//map messages by type 1: Ready; 2: Echo; 3: CallHelp; 4: Help
		switch rbcmsg.Type {
		case 1:
			//get a ready msg
			if rbcmsg.Round >= int32(bcl.height) {
				/*11*/ bcl.log.Println("get a ready msg of height", rbcmsg.Round, "from ", rbcmsg.ID)
				bcl.readyCH <- rbcmsg
				/*11*/ bcl.log.Println("done get a ready msg of height", rbcmsg.Round, "from ", rbcmsg.ID)
			}

		case 2:
			//get a echo msg
			if rbcmsg.Round >= int32(bcl.height) {
				/*11*/ bcl.log.Println("get a echo msg of height", rbcmsg.Round, "from ", rbcmsg.ID)
				bcl.echoCH <- rbcmsg
				/*11*/ bcl.log.Println("done get a echo msg of height", rbcmsg.Round, "from ", rbcmsg.ID)
			}
		case 3:
			//get a callhelp msg
			/*11*/
			/*11*/
			bcl.log.Println("get a callhelp msg of height", rbcmsg.Round, "from ", rbcmsg.ID)
			bcl.callhelpCH <- rbcmsg
		case 4:
		default:
			bcl.log.Fatalln("get a wrong type msg")
		}

	}

}

func (bcl *BC_l) Start() {
	bcl.log.Println(bcl.nid, "start broadcast leader ", bcl.nid, bcl.lid, bcl.sid)
	go bcl.handle_msgin()
	go bcl.handle_callhelp()
	//go bcl.handle_txin()
	var myhash []byte

	readybuf := make(chan pb.WRBCMsg)
	echobuf := make(chan pb.WRBCMsg)
	//send first ready msg
	//input, txcount = bcl.getinput()
	//input := <-bcl.input
	//txcount = len(input) / 250
	//timestamp = time.Now().UnixNano()

	//hash := sha256.New()
	//hash.Write(input)
	//myhash = hash.Sum(nil)
	//go bcl.send_ready_firsttime(bcl.height-1, input)

	//send first echo msg
	//go bcl.send_echo(bcl.height, myhash)
	firsttime := true
	for {
		readysignal := make(chan bool, 2)
		outputsignal := make(chan bool, 1)
		close := make(chan bool)

		var input []byte

		if firsttime {
			input = <-bcl.input
			firsttime = false
		} else {
			select {
			case input = <-bcl.input:
			default:
			//	fmt.Println("wait for input:", time.Now())
				bcl.signal2tpCH <- make([]byte, 1)
				fmt.Println("signal")
				input = <-bcl.input
			//	fmt.Println("get input:", time.Now())
			}

		}

		timestamp := time.Now().UnixNano()
		txcount := len(input) / 253
		fmt.Println("input size:", txcount)

		hash := sha256.New()
		hash.Write(input)
		myhash = hash.Sum(nil)

		go bcl.sned_val(bcl.height, input)
		go bcl.send_echo(bcl.height, myhash)

		//wait for echo msg
		oldechobuf := echobuf
		echobuf = make(chan pb.WRBCMsg, 4000)
		go bcl.handle_echo(bcl.height, close, myhash, readysignal, oldechobuf, echobuf)

		//wait for ready msg
		oldreadybuf := readybuf
		readybuf = make(chan pb.WRBCMsg, 4000)
		go bcl.handle_ready(bcl.height, close, myhash, readysignal, outputsignal, oldreadybuf, readybuf)

		<-readysignal

		//store value
		//bcl.db.Put(IntToBytes(bcl.height), input)
		//send ready
		go bcl.send_ready(bcl.height, myhash)
		//output
		<-outputsignal
		backBlock := &pb.BCBlock{
			RawBC: &pb.RawBC{
				Height:    int32(bcl.height),
				Leader:    int32(bcl.lid),
				K:         int32(bcl.sid),
				Timestamp: timestamp,
				Txcount:   int32(txcount),
			},
			//Payload: oldinput,
		}
		backBlockByte, err := proto.Marshal(backBlock)
		if err != nil {
			bcl.log.Panic(err)
		}
		/*11*/ bcl.log.Println("generate an output of height:", bcl.height)
		bcl.output <- backBlockByte
		bcl.log.Println("generate an output of height:", bcl.height, "done")
		SafeClose(close)
		bcl.height++

	}

}

func (bcl *BC_l) sned_val(height int, value []byte) {
	/*11*/ bcl.log.Println("send val msg of height", height, value[:10])
	msg := pb.WRBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		K:      int32(bcl.sid),
		Round:  int32(height),
		Type:   5,
		Value:  value,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	for i := 0; i < bcl.num; i++ {
		if i+1 != bcl.nid {
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcl.msgOut <- sendmsg
		}
	}
	/*11*/ bcl.log.Println("send val msg of height", height, "done")
}

func (bcl *BC_l) send_ready(height int, hash []byte) {
	/*11*/ bcl.log.Println("send ready msg of height", height)
	msg := pb.WRBCMsg{
		ID:      int32(bcl.nid),
		Leader:  int32(bcl.lid),
		K:       int32(bcl.sid),
		Round:   int32(height),
		Type:    1,
		Content: hash,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	for i := 0; i < bcl.num; i++ {
		if i+1 != bcl.nid {
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcl.msgOut <- sendmsg
		} else {
			bcl.readyCH <- msg
		}

	}
	/*11*/ bcl.log.Println("send ready msg of height", height, "done")
}

func (bcl *BC_l) send_echo(height int, hash []byte) {
	msg := pb.WRBCMsg{
		ID:      int32(bcl.nid),
		Leader:  int32(bcl.lid),
		K:       int32(bcl.sid),
		Round:   int32(height),
		Type:    2,
		Content: hash,
	}
	//bcl.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	//check proto
	newmsg := pb.WRBCMsg{}
	err = proto.Unmarshal(msgbyte, &newmsg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	//bcl.log.Println("************", newmsg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	/*11*/ bcl.log.Println("send echo msg of height", height)
	for i := 0; i < bcl.num; i++ {

		if i+1 != bcl.nid {
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}

			bcl.msgOut <- sendmsg

		} else {
			bcl.echoCH <- msg
		}

	}
	/*11*/ bcl.log.Println("send echo msg of height", height, "done")
}

func (bcl *BC_l) handle_echo(height int, close chan bool, myhash []byte, readysignal chan bool, oldbuf chan pb.WRBCMsg, futurebuf chan pb.WRBCMsg) {
	echocount := 0
	for {

		var msg pb.WRBCMsg
		select {
		case <-close:
			return
		default:
			select {
			case <-close:
				return
			case msg = <-bcl.echoCH:
			case msg = <-oldbuf:
			}
		}
		//check msg round, for leader, ignore old msg
		if msg.Round == int32(height) {
			/*11*/ bcl.log.Println("handle echo msg of height", msg.Round, "from", msg.ID)
			//check validity of path
			hash := msg.Content
			//as a leader, echo msg must has the same root with mine
			if bytes.Equal(hash, myhash) {
				echocount++

				if echocount == bcl.threshold*2 { //not +1,as we ignore the echo msg from myself
					/*11*/ bcl.log.Println("get enough echo msg to send ready")
					readysignal <- true
					return
				}

			}

		} else if msg.Round > int32(height) {
			futurebuf <- msg
		}

	}

}

func (bcl *BC_l) handle_ready(height int, close chan bool, myhash []byte, readysignal chan bool, outputsignal chan bool, oldbuf chan pb.WRBCMsg, futurebuf chan pb.WRBCMsg) {
	readycount := 0
	for {
		var msg pb.WRBCMsg
		select {
		case <-close:
			return
		default:
			select {
			case <-close:
				return
			case msg = <-bcl.readyCH:
			case msg = <-oldbuf:
			}
		}

		if msg.Round == int32(height) {
			/*11*/ bcl.log.Println("handle ready msg of height", msg.Round, "from ", msg.ID)
			hash := msg.Content
			if bytes.Equal(hash, myhash) {
				readycount++
				if readycount == bcl.threshold+1 {
					/*11*/ bcl.log.Println("get f+1 ready msg")
					readysignal <- true
				}
				if readycount == bcl.threshold*2 {
					/*11*/ bcl.log.Println("get 2f+1 ready msg")
					outputsignal <- true
					return
				}
			}
		} else if msg.Round > int32(height) {
			futurebuf <- msg
		}
		//count times receiving with the same root, if == f+1, send ready msg

		//if ==2f+1, kill this process and wait for n-f response echo msg
	}
}

func (bcl BC_l) send_help(height int, id int, value []byte) {

	msg := pb.WRBCMsg{
		ID:      int32(bcl.nid),
		Leader:  int32(bcl.lid),
		K:       int32(bcl.sid),
		Round:   int32(height),
		Type:    4,
		Content: value,
	}
	//bcl.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	/*11*/ bcl.log.Println("send help msg of height", height)

	sendmsg := SendMsg{
		ID:      id,
		Type:    0,
		Content: msgbyte,
	}
	bcl.msgOut <- sendmsg

	/*11*/
	bcl.log.Println("send help msg of height", height, "done")
}

func (bcl BC_l) handle_callhelp() {
	for {
		callhelpmsg := <-bcl.callhelpCH
		/*11*/ bcl.log.Println("handle callhelp msg of height", callhelpmsg.Round, "from", callhelpmsg.ID)
		value, err := bcl.db.Get(IntToBytes(int(callhelpmsg.Round)))
		if err != nil {
			bcl.log.Panicln(err)
		}
		if value != nil {
			//generate help msg
			bcl.send_help(int(callhelpmsg.Round), int(callhelpmsg.ID), value)
		}
	}

}

func (bcl *BC_l) handle_txin() {
	for {
		if len(bcl.txbuff) < txbuffsize {
			tx := <-bcl.input
			bcl.txbuff = append(bcl.txbuff, tx)

		} else {
			time.Sleep(time.Millisecond * 50)
		}

	}
}

func (bcl *BC_l) getinput() ([]byte, int) {
	for {
		txlen := len(bcl.txbuff)
		if txlen > bcl.batchsize {
			txBlk := make([][]byte, bcl.batchsize)
			for i, _ := range txBlk {
				copy(txBlk[i], bcl.txbuff[i])
			}
			copy(txBlk, bcl.txbuff)
			txs := pb.TXs{
				Txs: txBlk,
			}
			txsBytes, err := proto.Marshal(&txs)
			if err != nil {
				panic(err)
			}
			bcl.txbuff = bcl.txbuff[bcl.batchsize:]
			return txsBytes, bcl.batchsize
		} else if txlen != 0 {
			txBlk := make([][]byte, txlen)
			for i, _ := range txBlk {
				copy(txBlk[i], bcl.txbuff[i])
			}
			txs := pb.TXs{
				Txs: txBlk,
			}
			txsBytes, err := proto.Marshal(&txs)
			if err != nil {
				panic(err)
			}
			bcl.txbuff = bcl.txbuff[txlen:]
			return txsBytes, txlen
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
}

