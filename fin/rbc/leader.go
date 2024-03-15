package reliablebroadcast

import (
	"fmt"
	"bytes"
	mt "dumbo_fabric/crypto/merkle-tree"
	rs "dumbo_fabric/crypto/reed-solomon"
	pb "dumbo_fabric/struct"

	"github.com/golang/protobuf/proto"
)

var txbuffsize int = 10000

func NewBroadcast_leader(nid int, lid int, sid int, num int, input []byte, output chan pb.RBCOut, msgIn chan pb.RBCMsg, msgOut chan pb.SendMsg, height int, done chan bool) BC_l {
	newBC_l := BC_l{
		nid:       nid,
		lid:       lid,
		sid:       sid,
		num:       num,
		threshold: (num - 1) / 3,
		height:    height,
		//batchsize: batchsize,

		input:  input,
		output: output,
		//txbuff: make([][]byte, 0),
		msgIn:  msgIn,
		msgOut: msgOut,

		readyCH: make(chan pb.RBCMsg, 2000),
		echoCH:  make(chan pb.RBCMsg, 2000),

		done: done,
	}
	return newBC_l

}

//messages router
func (bcl *BC_l) handle_msgin(close chan bool) {
	//fmt.Println("inside handle_msgin")
	var rbcmsg pb.RBCMsg
	for {
		select {
		case <-close:
			return
		case <-bcl.done:
			return
		default:
			select {
			case <-close:
				return
			case <-bcl.done:
				return

			case rbcmsg = <-bcl.msgIn:
			}
		}

		//map messages by type 1: Ready; 2: Echo; 3: CallHelp; 4: Help
		switch rbcmsg.Type {
		case 1:
			//get a ready msg
			if rbcmsg.Round >= int32(bcl.height) {
				//fmt.Println("lid ", bcl.lid, "get a ready msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
				bcl.readyCH <- rbcmsg
			}
		case 2:
			//get a echo msg
			if rbcmsg.Round >= int32(bcl.height) {
				//fmt.Println("lid ", bcl.lid, "get a echo msg from ", rbcmsg.ID, "of height", rbcmsg.Round)
				bcl.echoCH <- rbcmsg
			}
		default:
			panic("get a wrong type msg")
		}

	}

}

var timestamp int64
var input []byte
var txcount int

func (bcl *BC_l) Start() {
	//fmt.Println(bcl.nid, "start broadcast leader ", bcl.nid, bcl.lid, bcl.sid)
	close := make(chan bool)
	go bcl.handle_msgin(close)
	//go bcl.handle_txin()

	readybuf := make(chan pb.RBCMsg)
	echobuf := make(chan pb.RBCMsg)
	//send first ready msg
	input := bcl.input
	//input, txcount = bcl.getinput()
	bcl.send_val(bcl.height, input)

	//send first echo msg
	go bcl.send_echo(len(input), bcl.mypath, bcl.height)

	readysignal := make(chan bool, 2)
	outputsignal := make(chan bool, 1)
	mypathCH := make(chan [][]byte, 1)
	inputCH := make(chan []byte, 1)
	myroot := bcl.mypath[len(bcl.mypath)-1]
	//wait for echo msg
	oldechobuf := echobuf
	echobuf = make(chan pb.RBCMsg, 2000)
	go bcl.handle_echo(bcl.height, close, myroot, readysignal, oldechobuf, echobuf)

	//wait for ready msg
	oldreadybuf := readybuf
	readybuf = make(chan pb.RBCMsg, 2000)
	go bcl.handle_ready(bcl.height, close, myroot, readysignal, outputsignal, oldreadybuf, readybuf)

	<-readysignal
	//oldtxcount := txcount
	go bcl.send_ready(bcl.height, myroot, mypathCH, inputCH)
	//output
	<-outputsignal
	bcl.output <- pb.RBCOut{input, bcl.lid}

	//fmt.Println("generate an output of height:", bcl.height)
	SafeClose(close)

}

func (bcl *BC_l) send_ready(height int, root []byte, mypathCH chan [][]byte, inputCH chan []byte) {

	//fmt.Println("lid ", bcl.lid, "send ready msg of height", height)
	for i := 0; i < bcl.num; i++ {

		if i+1 == bcl.nid {
			bcl.readyCH <- pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				K:      int32(bcl.sid),
				Round:  int32(height),
				Type:   1,
				Msglen: int32(len(input)),
				Root:   root,
			}
		} else {
			msg := pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				K:      int32(bcl.sid),
				Round:  int32(height),
				Type:   1,
				Msglen: int32(len(input)),
				Root:   root,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				panic(err)
			}
			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 1,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		}

	}
	//fmt.Println("send ready msg of height", height, "done")

}

func (bcl *BC_l) send_val(height int, input []byte) {

	//first time broadcast an input

	encoder := rs.New(bcl.threshold+1, bcl.num)
	shards := encoder.Encode(input)

	fmt.Println("input size:", len(input))

	lencount := 0
	for _, shard := range shards {
		lencount += len(shard)
	}

	fmt.Println("encode size:", lencount)

	merkletree, err := mt.NewTree(shards)
	if err != nil {
		panic(err)
	}

	//fmt.Println("lid ", bcl.lid, "send val msg of height", height)
	for i := 0; i < bcl.num; i++ {
		path, err := merkletree.GetMerklePath(shards[i])
		if err != nil {
			panic(err)
		}
		if i+1 == bcl.nid {
			bcl.mypath = path
		} else {
			msg := pb.RBCMsg{
				ID:     int32(bcl.nid),
				Leader: int32(bcl.lid),
				K:      int32(bcl.sid),
				Round:  int32(height),
				Type:   3,
				Msglen: int32(len(input)),
				Values: path,
			}
			msgbyte, err := proto.Marshal(&msg)
			if err != nil {
				panic(err)
			}

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 1,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		}

	}

	//fmt.Println("send ready msg of height", height, "done")

}

func (bcl *BC_l) send_echo(length int, path [][]byte, height int) {
	msg := pb.RBCMsg{
		ID:     int32(bcl.nid),
		Leader: int32(bcl.lid),
		K:      int32(bcl.sid),
		Round:  int32(height),
		Type:   2,
		Msglen: int32(length),
		Values: path,
	}
	//fmt.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	for i := 0; i < bcl.num; i++ {

		if i+1 != bcl.nid {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 1,
				Msg:  msgbyte,
			}
			bcl.msgOut <- sendmsg
		} else {
			bcl.echoCH <- msg
		}

	}
	//fmt.Println("send echo msg of height", height, "done")
}

func (bcl *BC_l) handle_echo(height int, close chan bool, myroot []byte, readysignal chan bool, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg) {
	echocount := 0
	for {

		var msg pb.RBCMsg
		select {
		case <-close:
			return
		case <-bcl.done:
			return
		default:
			select {
			case <-close:
				return
			case <-bcl.done:
				return
			case msg = <-bcl.echoCH:
			case msg = <-oldbuf:
			}
		}
		//check msg round, for leader, ignore old msg
		if msg.Round == int32(height) {
			//fmt.Println("handle echo msg from ", msg.ID, "of height", msg.Round)
			//check validity of path
			path := msg.Values
			root := path[len(path)-1]
			//as a leader, echo msg must has the same root with mine
			if bytes.Equal(root, myroot) {
				//to be done: fix bug in verifypath
				/*if mt.VerifyPath(root, path, int(msg.ID)-1) {
					echocount++

					if echocount == bcl.threshold*2 { //not +1,as we ignore the echo msg from myself
						//fmt.Println("get enough echo msg to send ready")
						readysignal <- true
						return
					}

				} else {
					panic("get a wrong path from ")
				}*/
				mt.VerifyPath(root, path, int(msg.ID)-1)
				echocount++

				if echocount == bcl.threshold*2 { //not +1,as we ignore the echo msg from myself
					//fmt.Println("get enough echo msg to send ready")
					readysignal <- true
					return
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
		case <-bcl.done:
			return
		default:
			select {
			case <-close:
				return
			case <-bcl.done:
				return
			case msg = <-bcl.readyCH:
			case msg = <-oldbuf:
			}
		}

		if msg.Round == int32(height) {
			//fmt.Println("handle ready msg from ", msg.ID, "of height", msg.Round)
			root := msg.Root
			if bytes.Equal(root, myroot) {
				readycount++
				if readycount == bcl.threshold+1 {
					//fmt.Println("get f+1 ready msg")
					readysignal <- true
				}
				if readycount == bcl.threshold*2 {
					//fmt.Println("get 2f+1 ready msg")
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

