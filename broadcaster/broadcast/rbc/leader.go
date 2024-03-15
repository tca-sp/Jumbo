package reliablebroadcast

import (
	"bytes"
	mt "dumbo_fabric/crypto/merkle-tree"
	rs "dumbo_fabric/crypto/reed-solomon"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
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

		readyCH: make(chan pb.RBCMsg, 2000),
		echoCH:  make(chan pb.RBCMsg, 2000),

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
	var rbcmsg pb.RBCMsg
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
				//bcl.log.Println("get a ready msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
				bcl.readyCH <- rbcmsg
			}
		case 2:
			//get a echo msg
			if rbcmsg.Round >= int32(bcl.height) {
				//bcl.log.Println("get a echo msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
				bcl.echoCH <- rbcmsg
			}
		case 3:

		case 4:
		default:
			bcl.log.Fatalln("get a wrong type msg")
		}

	}

}

var timestamp int64
var input []byte
var txcount int

func (bcl *BC_l) Start() {
	bcl.log.Println(bcl.nid, "start broadcast leader ", bcl.nid, bcl.lid, bcl.sid)
	go bcl.handle_msgin()
	//go bcl.handle_txin()
	var mypath [][]byte

	readybuf := make(chan pb.RBCMsg)
	echobuf := make(chan pb.RBCMsg)
	//send first ready msg
	input := <-bcl.input
	timestamp = time.Now().UnixNano()
	//input, txcount = bcl.getinput()
	mypath = bcl.send_ready_firsrtime(bcl.height-1, input, nil)

	//send first echo msg
	go bcl.send_echo(len(input), mypath, bcl.height)
	for {
		readysignal := make(chan bool, 2)
		outputsignal := make(chan bool, 1)
		mypathCH := make(chan [][]byte, 1)
		inputCH := make(chan []byte, 1)
		close := make(chan bool)
		myroot := mypath[len(mypath)-1]
		//wait for echo msg
		oldechobuf := echobuf
		echobuf = make(chan pb.RBCMsg, 2000)
		go bcl.handle_echo(bcl.height, close, myroot, readysignal, oldechobuf, echobuf)

		//wait for ready msg
		oldreadybuf := readybuf
		readybuf = make(chan pb.RBCMsg, 2000)
		go bcl.handle_ready(bcl.height, close, myroot, readysignal, outputsignal, oldreadybuf, readybuf)

		<-readysignal
		//send ready
		oldinput := input
		oldtimestamp := timestamp
		//oldtxcount := txcount
		go bcl.send_ready(bcl.height, myroot, mypathCH, inputCH)
		//output
		<-outputsignal
		if !bcl.testmode {
			bcl.db.Put(IntToBytes(bcl.height), oldinput)
		}

		backBlock := &pb.BCBlock{
			RawBC: &pb.RawBC{
				Height:    int32(bcl.height),
				Leader:    int32(bcl.lid),
				K:         int32(bcl.sid),
				Timestamp: oldtimestamp,
				//Txcount:   int32(oldtxcount),
			},
			//Payload: oldinput,
		}
		backBlockByte, _ := proto.Marshal(backBlock)
		bcl.output <- backBlockByte
		bcl.log.Println("generate an output of height:", bcl.height, timestamp)
		SafeClose(close)
		bcl.height++
		input = <-inputCH
		mypath = <-mypathCH
		go bcl.send_echo(len(input), mypath, bcl.height)

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

func (bcl *BC_l) send_ready(height int, root []byte, mypathCH chan [][]byte, inputCH chan []byte) {
	//input, txcount = bcl.getinput()
	var input []byte
	select {
	case input = <-bcl.input:
	default:
		bcl.signal2tpCH <- make([]byte, 1)
		input = <-bcl.input
	}
	timestamp = time.Now().UnixNano()
	inputCH <- input

	//first time broadcast an input

	encoder := rs.New(bcl.threshold+1, bcl.num)
	shards := encoder.Encode(input)

	merkletree, err := mt.NewTree(shards)
	if err != nil {
		bcl.log.Fatalln(err)
	}

	//bcl.log.Println("send ready msg of height", height)
	for i := 0; i < bcl.num; i++ {
		path, err := merkletree.GetMerklePath(shards[i])
		if err != nil {
			bcl.log.Fatalln(err)
		}
		if i+1 == bcl.nid {
			mypathCH <- path
		} else {
			msg := pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				K:      int32(bcl.sid),
				Round:  int32(height),
				Type:   1,
				Msglen: int32(len(input)),
				Root:   root,
				Values: path,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				bcl.log.Fatalln(err)
			}
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcl.msgOut <- sendmsg
		}

	}
	//bcl.log.Println("send ready msg of height", height, "done")

}

func (bcl *BC_l) send_ready_firsrtime(height int, input []byte, root []byte) [][]byte {
	var mypath [][]byte
	//first time broadcast an input

	encoder := rs.New(bcl.threshold+1, bcl.num)
	shards := encoder.Encode(input)

	merkletree, err := mt.NewTree(shards)
	if err != nil {
		bcl.log.Fatalln(err)
	}

	//bcl.log.Println("send ready msg of height", height)
	for i := 0; i < bcl.num; i++ {
		path, err := merkletree.GetMerklePath(shards[i])
		if err != nil {
			bcl.log.Fatalln(err)
		}
		if i+1 == bcl.nid {
			mypath = path
		} else {
			msg := pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				K:      int32(bcl.sid),
				Round:  int32(height),
				Type:   1,
				Msglen: int32(len(input)),
				Root:   root,
				Values: path,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				bcl.log.Fatalln(err)
			}
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcl.msgOut <- sendmsg
		}

	}
	//bcl.log.Println("send ready msg of height", height, "done")

	return mypath
}

func (bcl *BC_l) send_echo(len int, path [][]byte, height int) {
	msg := pb.RBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		K:      int32(bcl.sid),
		Round:  int32(height),
		Type:   2,
		Msglen: int32(len),
		Values: path,
	}
	//bcl.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	//check proto
	newmsg := pb.RBCMsg{}
	err = proto.Unmarshal(msgbyte, &newmsg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	//bcl.log.Println("************", newmsg)
	if err != nil {
		bcl.log.Fatalln(err)
	}
	//bcl.log.Println("send echo msg of height", height)
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
	//bcl.log.Println("send echo msg of height", height, "done")
}

func (bcl *BC_l) handle_echo(height int, close chan bool, myroot []byte, readysignal chan bool, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	echocount := 0
	for {

		var msg pb.RBCMsg
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
			//bcl.log.Println("handle echo msg from ", msg.ID, "of height", msg.Round)
			//check validity of path
			path := msg.Values
			root := path[len(path)-1]
			//as a leader, echo msg must has the same root with mine
			if bytes.Equal(root, myroot) {
				if mt.VerifyPath(root, path, int(msg.ID)-1) {
					echocount++

					if echocount == bcl.threshold*2 { //not +1,as we ignore the echo msg from myself
						//bcl.log.Println("get enough echo msg to send ready")
						readysignal <- true
						return
					}

				} else {
					bcl.log.Fatalln("get a wrong path from ", msg.ID)
				}
			}

		} else if msg.Round > int32(height) {
			futurebuf <- msg
		}

	}

}

func (bcl *BC_l) handle_ready(height int, close chan bool, myroot []byte, readysignal chan bool, outputsignal chan bool, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	readycount := 0
	for {
		var msg pb.RBCMsg
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
			//bcl.log.Println("handle ready msg from ", msg.ID, "of height", msg.Round)
			root := msg.Root
			if bytes.Equal(root, myroot) {
				readycount++
				if readycount == bcl.threshold+1 {
					//bcl.log.Println("get f+1 ready msg")
					readysignal <- true
				}
				if readycount == bcl.threshold*2 {
					//bcl.log.Println("get 2f+1 ready msg")
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
