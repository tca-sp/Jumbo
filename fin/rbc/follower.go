package reliablebroadcast

import (
	"bytes"
	mt "dumbo_fabric/crypto/merkle-tree"
	rs "dumbo_fabric/crypto/reed-solomon"
	pb "dumbo_fabric/struct"
	//"fmt"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_follower(nid int, lid int, sid int, num int, output chan pb.RBCOut, msgIn chan pb.RBCMsg, msgOut chan pb.SendMsg, height int, done chan bool) BC_f {

	newBC_f := BC_f{
		nid:       nid,
		lid:       lid,
		sid:       sid,
		num:       num,
		threshold: (num - 1) / 3,
		height:    height,

		output: output,
		msgIn:  msgIn,
		msgOut: msgOut,

		valCH:   make(chan pb.RBCMsg, 100),
		readyCH: make(chan pb.RBCMsg, 2000),
		echoCH:  make(chan pb.RBCMsg, 2000),

		done: done,
	}
	return newBC_f

}

//messages router
func (bcf *BC_f) handle_msgin(close chan bool) {
	//fmt.Println("inside handle_msgin")
	var rbcmsg pb.RBCMsg
	for {
		select {
		case <-close:
			return
		case <-bcf.done:
			return
		default:
			select {

			case <-close:
				return
			case <-bcf.done:
				return

			case rbcmsg = <-bcf.msgIn:
			}
		}

		//map messages by type 1: Ready; 2: Echo; 3: CallHelp; 4: Help
		switch rbcmsg.Type {
		case 1:
			//get a ready msg
			//fmt.Println("lid ", bcf.lid, "get a ready msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			if rbcmsg.Round == int32(bcf.height) {
				bcf.readyCH <- rbcmsg
			}
		case 2:
			//get a echo msg
			if rbcmsg.Round == int32(bcf.height) {
				//fmt.Println("lid ", bcf.lid, "get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.echoCH <- rbcmsg
			}
		case 3:
			if rbcmsg.Round == int32(bcf.height) {
				//fmt.Println("lid ", bcf.lid, "get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.valCH <- rbcmsg
			}
		case 4:
		default:
			panic("get a wrong type msg")
		}

	}

}

type candidate struct {
	root   []byte
	output []byte
}

func (bcf *BC_f) Start() {
	//fmt.Println(bcf.nid, "start broadcast follower ", bcf.nid, bcf.lid, bcf.sid)
	close := make(chan bool)
	go bcf.handle_msgin(close)
	valbuf := make(chan pb.RBCMsg)
	oldechobuf := make(chan pb.RBCMsg)
	oldreadybuf := make(chan pb.RBCMsg)
	readybuf := make(chan pb.RBCMsg)
	echobuf := make(chan pb.RBCMsg)

	readysignal := make(chan []byte, 2)
	outputsignal := make(chan []byte, 1)
	candidateCH := make(chan candidate, 2)

	//wait for val msg
	oldvalbuf := valbuf
	valbuf = make(chan pb.RBCMsg, 100)
	go bcf.handle_val(bcf.height, oldvalbuf, valbuf, close)

	//wait for echo msg
	check_rbcbuf(oldechobuf, echobuf, bcf.height)
	oldechobuf = echobuf
	echobuf = make(chan pb.RBCMsg, 2000)
	go bcf.handle_echo(bcf.height, candidateCH, readysignal, oldechobuf, echobuf, close)

	//wait for ready msg
	check_rbcbuf(oldreadybuf, readybuf, bcf.height)
	oldreadybuf = readybuf
	readybuf = make(chan pb.RBCMsg, 2000)
	go bcf.handle_ready(bcf.height, readysignal, outputsignal, oldreadybuf, readybuf, close)

	//fmt.Println("lid", bcf.lid, "wait ready")
	readyroot := <-readysignal
	//fmt.Println("lid", bcf.lid, "send ready")
	//send ready
	go bcf.send_ready(bcf.height, readyroot)
	//output

	//fmt.Println("lid", bcf.lid, "wait output")
	outroot := <-outputsignal

	//fmt.Println("lid", bcf.lid, "done")
	for {
		cand := <-candidateCH
		if bytes.Equal(outroot, cand.root) {
			bcf.output <- pb.RBCOut{cand.output, bcf.lid}
			//fmt.Println("generate an output of height:", bcf.height)
			break
		} else {
			panic("wrong leader")
		}
	}
	SafeClose(close)

}

func check_rbcbuf(old chan pb.RBCMsg, new chan pb.RBCMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.Round >= int32(height) {
			new <- oldmsg
		}
	}
}

func (bcf *BC_f) handle_val(height int, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg, close chan bool) {
	//fmt.Println("inside handle_val")
	for {
		var msg pb.RBCMsg
		select {
		case <-close:
			return
		case <-bcf.done:
			return
		default:
			select {
			case <-close:
				return
			case <-bcf.done:
				return
			case msg = <-bcf.valCH:
			case msg = <-oldbuf:
			}
		}
		if msg.Round == int32(height) {
			//fmt.Println("handle val msg from", msg.ID, " of round ", height)
			//check path
			root := msg.Values[len(msg.Values)-1]
			//to be done: fix bug in verifypath
			/*if mt.VerifyPath(root, msg.Values, bcf.nid-1) {
				bcf.send_echo(int(msg.Msglen), height, msg.Values)
			} else {
				panic("get a wrong path from leader")
			}*/
			mt.VerifyPath(root, msg.Values, bcf.nid-1)
			bcf.send_echo(int(msg.Msglen), height, msg.Values)
		} else if msg.Round > int32(height)-1 {
			futurebuf <- msg
		}
	}
}

type echobuf struct {
	count     int
	shards    [][]byte
	rootvalid bool
}

func (bcf *BC_f) handle_echo(height int, candidateCH chan candidate, readysignal chan []byte, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg, closeCH chan bool) {
	//fmt.Println("inside handle_echo")
	encoder := rs.New(bcf.threshold+1, bcf.num)
	echomap := make(map[string]*echobuf)
	echocount := 0
	for {

		var msg pb.RBCMsg
		select {
		case <-closeCH:
			//close(oldbuf)
			//routinechannel(oldbuf, bcf.echoCH, height)
			return
		case <-bcf.done:
			return
		default:
			select {
			case <-closeCH:
				//close(oldbuf)
				//routinechannel(oldbuf, bcf.echoCH, height)
				return
			case <-bcf.done:
				return
			case msg = <-bcf.echoCH:
			case msg = <-oldbuf:
			}
		}

		//check msg round, for leader, ignore future and old msg
		if msg.Round == int32(height) {
			echocount++
			//fmt.Println("handle echo msg of round ", height, "from", msg.ID)
			//check validity of path
			//fmt.Println(msg)
			path := msg.Values
			root := path[len(path)-1]
			//to be done: fix bug in verifypath
			flag := mt.VerifyPath(root, path, int(msg.ID)-1)
			flag = true
			if !flag {
				panic("wrong path")
			}
			if flag {
				//store this path in map<root,shard>
				_, ok := echomap[string(root)]
				if ok {
					echomap[string(root)].shards[msg.ID-1] = path[0]
					echomap[string(root)].count++
				} else {
					echomap[string(root)] = &echobuf{1, make([][]byte, bcf.num), false}
					echomap[string(root)].shards[msg.ID-1] = path[0]
				}

				//check if this root has been received f+1 times, if has, generate a payback
				if echomap[string(root)].count == bcf.threshold+1 {
					//reconstruct origin msg and recompute the root, and check identity.
					//fmt.Println("reconstruct input")
					cand := encoder.Reconstruct(echomap[string(root)].shards, int(msg.Msglen))

					//check merkle root
					tree, err := mt.NewTree(echomap[string(root)].shards)
					if err != nil {
						panic(err)
					}
					if bytes.Equal(tree.MerkleRoot(), root) {
						echomap[string(root)].rootvalid = true
						//fmt.Println("reconstruct input working")
						candidateCH <- candidate{root, cand}
						//fmt.Println("reconstruct input done")
					} else {
						panic("wrong leader")
					}

				}

				//check if this root has n-f shards, if has,  send ready msg
				if echomap[string(root)].count == bcf.threshold*2+1 {
					if echomap[string(root)].rootvalid {
						readysignal <- root
						//fmt.Println("ready to send ready in echo")
						return
					} else {
						panic("wrong leader")
					}
				}

			}

		} else if msg.Round > int32(height) {
			futurebuf <- msg
		}

		if echocount == bcf.num {
			return
		}

	}

}

func (bcf *BC_f) handle_ready(height int, readysignal chan []byte, outputsignal chan []byte, oldbuf chan pb.RBCMsg, futurebuf chan pb.RBCMsg, closeCH chan bool) {

	//fmt.Println("inside handle_ready")
	readymap := make(map[string]int)
	for {
		var msg pb.RBCMsg
		select {
		case <-closeCH:
			//close(oldbuf)
			//routinechannel(oldbuf, bcf.readyCH, height)
			return
		case <-bcf.done:
			return
		default:
			select {
			case <-closeCH:
				//close(oldbuf)
				//routinechannel(oldbuf, bcf.readyCH, height)
				return
			case <-bcf.done:
				return
			case msg = <-bcf.readyCH:
			case msg = <-oldbuf:
			}
		}
		if msg.Round > int32(height) {
			futurebuf <- msg
		} else if msg.Round == int32(height) {
			//fmt.Println("handle ready msg from", msg.ID, " of round ", height)

			_, ok := readymap[string(msg.Root)]
			if ok {
				readymap[string(msg.Root)]++
			} else {
				readymap[string(msg.Root)] = 1
			}

			//count times receiving with the same root, if == f+1, send ready msg
			if readymap[string(msg.Root)] == bcf.threshold+1 {
				readysignal <- msg.Root
				//fmt.Println("ready to send ready in ready")
			}

			//if ==2f+1, kill this process and wait for n-f response echo msg
			if readymap[string(msg.Root)] == bcf.threshold*2+1 {
				outputsignal <- msg.Root
				//fmt.Println("ready to output")
				return
			}
		}

	}
}

func (bcf *BC_f) send_ready(height int, root []byte) {
	//fmt.Println("lid ", bcf.lid, "send ready msg of height ", height)
	for i := 0; i < bcf.num; i++ {

		if i+1 == bcf.nid {
			bcf.readyCH <- pb.RBCMsg{
				ID:     int32(bcf.nid),
				Leader: int32(bcf.lid),
				K:      int32(bcf.sid),
				Round:  int32(height),
				Type:   1,
				Root:   root,
			}
		} else {
			msg := pb.RBCMsg{
				ID:     int32(bcf.nid),
				Leader: int32(bcf.lid),
				K:      int32(bcf.sid),
				Round:  int32(height),
				Type:   1,
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
			bcf.msgOut <- sendmsg
		}

	}
	//fmt.Println("send ready msg of height", height, "done")
}

func (bcf *BC_f) send_echo(len int, height int, path [][]byte) {
	msg := pb.RBCMsg{
		ID:     int32(bcf.nid),
		Leader: int32(bcf.lid),
		K:      int32(bcf.sid),
		Round:  int32(height),
		Type:   2,
		Msglen: int32(len),
		Values: path,
	}
	//fmt.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		panic(err)
	}
	//fmt.Println("lid ", bcf.lid, "send echo msg of height", height)
	for i := 0; i < bcf.num; i++ {

		if i+1 != bcf.nid {

			sendmsg := pb.SendMsg{
				ID:   i + 1,
				Type: 1,
				Msg:  msgbyte,
			}
			bcf.msgOut <- sendmsg
		} else {
			bcf.echoCH <- msg
		}

	}
	//fmt.Println("send echo msg of height", height, "done")
}

func routinechannel(old chan pb.RBCMsg, new chan pb.RBCMsg, height int) {
	for oldmsg := range old {
		if oldmsg.Round > int32(height) {
			new <- oldmsg
		}
	}
}

