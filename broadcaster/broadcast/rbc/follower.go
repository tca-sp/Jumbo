package reliablebroadcast

import (
	"bytes"
	mt "dumbo_fabric/crypto/merkle-tree"
	rs "dumbo_fabric/crypto/reed-solomon"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"log"

	"github.com/golang/protobuf/proto"
)

func NewBroadcast_follower(nid int, lid int, sid int, num int, output chan []byte, msgIn chan []byte, msgOut chan SendMsg, log log.Logger, db leveldb.DB, testmode bool) BC_f {

	newBC_f := BC_f{
		nid:       nid,
		lid:       lid,
		sid:       sid,
		num:       num,
		threshold: (num - 1) / 3,
		height:    0,

		output: output,
		msgIn:  msgIn,
		msgOut: msgOut,

		valCH:   make(chan pb.RBCMsg, 100),
		readyCH: make(chan pb.RBCMsg, 2000),
		echoCH:  make(chan pb.RBCMsg, 2000),

		log:      log,
		db:       db,
		testmode: testmode,
	}
	return newBC_f

}

//messages router
func (bcf *BC_f) handle_msgin() {
	bcf.log.Println("inside handle_msgin")
	var rbcmsg pb.RBCMsg
	var rbcmsgByte []byte
	for {

		rbcmsgByte = <-bcf.msgIn
		err := proto.Unmarshal(rbcmsgByte, &rbcmsg)
		if err != nil {
			bcf.log.Fatalln(err)
		}

		//map messages by type 1: Ready; 2: Echo; 3: CallHelp; 4: Help
		switch rbcmsg.Type {
		case 1:
			//get a ready msg
			//bcf.log.Println("get a ready msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			if rbcmsg.Round >= int32(bcf.height) || rbcmsg.ID == int32(bcf.lid) {
				bcf.readyCH <- rbcmsg
				if rbcmsg.ID == int32(bcf.lid) {
					//bcf.log.Println("get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
					bcf.valCH <- rbcmsg
				}
			}
		case 2:
			//get a echo msg
			if rbcmsg.Round >= int32(bcf.height) {
				//bcf.log.Println("get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.echoCH <- rbcmsg
			}
		case 3:

		case 4:
		default:
			bcf.log.Fatalln("get a wrong type msg")
		}

	}

}

type candidate struct {
	root   []byte
	output []byte
}

func (bcf *BC_f) Start() {
	bcf.log.Println(bcf.nid, "start broadcast follower ", bcf.nid, bcf.lid, bcf.sid)
	go bcf.handle_msgin()
	valbuf := make(chan pb.RBCMsg)
	oldechobuf := make(chan pb.RBCMsg)
	oldreadybuf := make(chan pb.RBCMsg)
	readybuf := make(chan pb.RBCMsg)
	echobuf := make(chan pb.RBCMsg)
	for {
		readysignal := make(chan []byte, 2)
		outputsignal := make(chan []byte, 1)
		close := make(chan bool)
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

		readyroot := <-readysignal
		//send ready
		go bcf.send_ready(bcf.height, readyroot)
		//output
		outroot := <-outputsignal
		for {
			cand := <-candidateCH
			if bytes.Equal(outroot, cand.root) {
				if !bcf.testmode {
					bcf.db.Put(IntToBytes(bcf.height), cand.output)
				}
				/*txblk := pb.TXs{}
				err := proto.Unmarshal(cand.output, &txblk)
				if err != nil {
					bcf.log.Fatalln(err)
				}*/

				backBlock := &pb.BCBlock{
					RawBC: &pb.RawBC{
						Height: int32(bcf.height),
						Leader: int32(bcf.lid),
						K:      int32(bcf.sid),
						//Timestamp: txblk.Timestamp,
						//Txcount:   int32(len(txblk.Txs)),
					},
					//Payload: cand.output,
				}
				backBlockByte, _ := proto.Marshal(backBlock)
				bcf.output <- backBlockByte
				bcf.log.Println("generate an output of height:", bcf.height)
				break
			} else {
				bcf.log.Fatalln("wrong leader")
			}
		}
		SafeClose(close)
		bcf.height++

	}

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
	bcf.log.Println("inside handle_val")
	for {
		var msg pb.RBCMsg
		select {
		case <-close:
			return
		default:
			select {
			case <-close:
				return
			case msg = <-bcf.valCH:
			case msg = <-oldbuf:
			}
		}
		if msg.Round == int32(height)-1 {
			//bcf.log.Println("handle val msg from", msg.ID, " of round ", height)
			//check path
			root := msg.Values[len(msg.Values)-1]
			if mt.VerifyPath(root, msg.Values, bcf.nid-1) {
				bcf.send_echo(int(msg.Msglen), height, msg.Values)
			} else {
				bcf.log.Fatalln("get a wrong path from leader")
			}
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
	bcf.log.Println("inside handle_echo")
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
		default:
			select {
			case <-closeCH:
				//close(oldbuf)
				//routinechannel(oldbuf, bcf.echoCH, height)
				return
			case msg = <-bcf.echoCH:
			case msg = <-oldbuf:
			}
		}

		//check msg round, for leader, ignore future and old msg
		if msg.Round == int32(height) {
			echocount++
			//bcf.log.Println("handle echo msg of round ", height, "from", msg.ID)
			//check validity of path
			//bcf.log.Println(msg)
			path := msg.Values
			root := path[len(path)-1]
			flag := mt.VerifyPath(root, path, int(msg.ID)-1)
			if !flag {
				bcf.log.Fatalln("wrong path")
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
					//bcf.log.Println("reconstruct input")
					cand := encoder.Reconstruct(echomap[string(root)].shards, int(msg.Msglen))

					//check merkle root
					tree, err := mt.NewTree(echomap[string(root)].shards)
					if err != nil {
						bcf.log.Fatalln(err)
					}
					if bytes.Equal(tree.MerkleRoot(), root) {
						echomap[string(root)].rootvalid = true
						//bcf.log.Println("reconstruct input working")
						candidateCH <- candidate{root, cand}
						//bcf.log.Println("reconstruct input done")
					} else {
						bcf.log.Fatalln("wrong leader")
					}

				}

				//check if this root has n-f shards, if has,  send ready msg
				if echomap[string(root)].count == bcf.threshold*2+1 {
					if echomap[string(root)].rootvalid {
						readysignal <- root
						//bcf.log.Println("ready to send ready in echo")
						return
					} else {
						bcf.log.Fatalln("wrong leader")
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

	bcf.log.Println("inside handle_ready")
	readymap := make(map[string]int)
	for {
		var msg pb.RBCMsg
		select {
		case <-closeCH:
			//close(oldbuf)
			//routinechannel(oldbuf, bcf.readyCH, height)
			return
		default:
			select {
			case <-closeCH:
				//close(oldbuf)
				//routinechannel(oldbuf, bcf.readyCH, height)
				return
			case msg = <-bcf.readyCH:
			case msg = <-oldbuf:
			}
		}
		if msg.Round > int32(height) {
			futurebuf <- msg
		} else if msg.Round == int32(height) {
			//bcf.log.Println("handle ready msg from", msg.ID, " of round ", height)

			_, ok := readymap[string(msg.Root)]
			if ok {
				readymap[string(msg.Root)]++
			} else {
				readymap[string(msg.Root)] = 1
			}

			//count times receiving with the same root, if == f+1, send ready msg
			if readymap[string(msg.Root)] == bcf.threshold+1 {
				readysignal <- msg.Root
				//bcf.log.Println("ready to send ready in ready")
			}

			//if ==2f+1, kill this process and wait for n-f response echo msg
			if readymap[string(msg.Root)] == bcf.threshold*2+1 {
				outputsignal <- msg.Root
				//bcf.log.Println("ready to output")
				return
			}
		}

	}
}

func (bcf *BC_f) send_ready(height int, root []byte) {
	//bcf.log.Println("send ready msg of height ", height)
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
				bcf.log.Fatalln(err)
			}
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcf.msgOut <- sendmsg
		}

	}
	//bcf.log.Println("send ready msg of height", height, "done")
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
	//bcf.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcf.log.Fatalln(err)
	}
	//bcf.log.Println("send echo msg of height", height)
	for i := 0; i < bcf.num; i++ {

		if i+1 != bcf.nid {

			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcf.msgOut <- sendmsg
		} else {
			bcf.echoCH <- msg
		}

	}
	//bcf.log.Println("send echo msg of height", height, "done")
}

func routinechannel(old chan pb.RBCMsg, new chan pb.RBCMsg, height int) {
	for oldmsg := range old {
		if oldmsg.Round > int32(height) {
			new <- oldmsg
		}
	}
}
