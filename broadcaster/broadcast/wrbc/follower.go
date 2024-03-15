package wrbc

import (
	"bytes"
	"crypto/sha256"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"log"
	"sort"

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

		valCH:       make(chan pb.WRBCMsg, 400),
		readyCH:     make(chan pb.WRBCMsg, 400),
		echoCH:      make(chan pb.WRBCMsg, 400),
		roundinfoCH: make(chan Roundinfo, 400),

		callhelpCH: make(chan pb.WRBCMsg, 400),
		helpCH:     make(chan pb.WRBCMsg, 400),

		//missMap:  make(map[int][]byte),
		//missMap: MissMap{sync.Mutex{}, make(map[int][]byte)},

		log: log,
		db:  db,

		testmode: testmode,
	}
	return newBC_f

}

//messages router
func (bcf *BC_f) handle_msgin() {
	bcf.log.Println("inside handle_msgin")
	var rbcmsg pb.WRBCMsg
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
			if rbcmsg.Round >= int32(bcf.height) {
				//**11 bcf.log.Println("get a ready msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.readyCH <- rbcmsg
				//**11 bcf.log.Println("done get a ready msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				//if rbcmsg.ID == int32(bcf.lid) {
				//	//**11 bcf.log.Println("get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				//	bcf.valCH <- rbcmsg
				//	//**11 bcf.log.Println("done get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				//}
			} //else if rbcmsg.ID == int32(bcf.lid) {
			//	//**11 bcf.log.Println("get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			//	bcf.valCH <- rbcmsg
			//	//**11 bcf.log.Println("done get a val msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			//}
		case 2:
			//get a echo msg
			if rbcmsg.Round >= int32(bcf.height) {
				//**11 bcf.log.Println("get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.echoCH <- rbcmsg
				//**11 bcf.log.Println("done get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			}
		case 3:
			//get a callhelp msg
			//**11 bcf.log.Println("get a callhelp msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			bcf.callhelpCH <- rbcmsg
		case 4:
			//get a help msg
			//**11 bcf.log.Println("get a help msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
			bcf.helpCH <- rbcmsg
		case 5:
		//	if rbcmsg.Round >= int32(bcf.height) {
				//**11 bcf.log.Println("get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
				bcf.valCH <- rbcmsg
				//**11 bcf.log.Println("done get a echo msg from ", rbcmsg.ID, "of height ", rbcmsg.Round)
		//	}
		default:
			bcf.log.Fatalln("get a wrong type msg")
		}

	}

}

func (bcf *BC_f) Start() {
	bcf.log.Println(bcf.nid, "start broadcast follower ", bcf.nid, bcf.lid, bcf.sid)
	go bcf.handle_msgin()
	go bcf.handle_val()
	//go bcf.handle_callhelp()
	//go bcf.handle_help()

	oldechobuf := make(chan pb.WRBCMsg)
	oldreadybuf := make(chan pb.WRBCMsg)
	readybuf := make(chan pb.WRBCMsg)
	echobuf := make(chan pb.WRBCMsg)
	for {
		readysignal := make(chan []byte, 10)
		outputsignal := make(chan []byte, 10)
		close := make(chan bool)

		//wait for echo msg
		bcf.check_rbcbuf(oldechobuf, echobuf, bcf.height)
		oldechobuf = echobuf
		echobuf = make(chan pb.WRBCMsg, 400)
		go bcf.handle_echo(bcf.height, readysignal, oldechobuf, echobuf, close)

		//wait for ready msg
		bcf.check_rbcbuf(oldreadybuf, readybuf, bcf.height)
		oldreadybuf = readybuf
		readybuf = make(chan pb.WRBCMsg, 400)
		go bcf.handle_ready(bcf.height, readysignal, outputsignal, oldreadybuf, readybuf, close)

		hash := <-readysignal
		//send ready
		go bcf.send_ready(bcf.height, hash)
		//output
		outputhash := <-outputsignal
		//**11 bcf.log.Println("generate a roundinfo of round", bcf.height, outputhash)
		bcf.roundinfoCH <- Roundinfo{int32(bcf.height), outputhash}

		//value := <-valueCH
		// bcf.log.Println("I have the value of height ", bcf.height)
		//hash1 := sha256.New()
		//hash1.Write(value)
		//valuehash := hash1.Sum(nil)
		//if !bytes.Equal(valuehash, outputhash) {
		//	bcf.log.Panic("wrong leader")
		//}
		//
		///*txblk := pb.TXs{}
		//err := proto.Unmarshal(value, &txblk)
		//if err != nil {
		//	bcf.log.Fatalln(err)
		//}*/
		//
		//backBlock := &pb.BCBlock{
		//	RawBC: &pb.RawBC{
		//		Height: int32(bcf.height),
		//		Leader: int32(bcf.lid),
		//		K:      int32(bcf.sid),
		//		//Timestamp: txblk.Timestamp,
		//		//Txcount:   int32(len(txblk.Txs)),
		//	},
		//	//Payload: cand.output,
		//}
		//backBlockByte, _ := proto.Marshal(backBlock)
		//bcf.output <- backBlockByte
		bcf.log.Println("done a round of height:", bcf.height)

		SafeClose(close)
		bcf.height++

	}

}

func (bcf *BC_f) check_rbcbuf(old chan pb.WRBCMsg, new chan pb.WRBCMsg, height int) {
	length := len(old)
	//**11 bcf.log.Println("old buff len", length)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.Round >= int32(height) {
			new <- oldmsg
		}
	}
}

func (bcf *BC_f) handle_val() {
	var valbuf []pb.WRBCMsg
	var roundinfobuf []Roundinfo
	var lastheight int32 = -1 //greatest height that has been commit
	for {
		select {
		case valmsg := <-bcf.valCH:
			//**11 bcf.log.Println("handle valmsg of round in handle_val", valmsg.Round, valmsg.Value)

			if valmsg.Round <= lastheight+1 {
				value := valmsg.Value
				hash := sha256.New()
				hash.Write(value)
				value_hash := hash.Sum(nil)
				go bcf.send_echo(int(valmsg.Round), value_hash)
			}

			valbuf = append(valbuf, valmsg)
			sort.SliceStable(valbuf, func(i int, j int) bool { return valbuf[i].Round < valbuf[j].Round })

			for {
				if len(roundinfobuf) > 0 && len(valbuf) > 0 {
					if valbuf[0].Round == roundinfobuf[0].height {
						hash := sha256.New()
						hash.Write(valbuf[0].Value)
						value_hash := hash.Sum(nil)
						if !bytes.Equal(value_hash, roundinfobuf[0].valuehash) {
							bcf.log.Fatalln("wrong leader 1")
						}
						//get a val we need,generate output and remove val and roundinfo
						backBlock := &pb.BCBlock{
							RawBC: &pb.RawBC{
								Height:  int32(roundinfobuf[0].height),
								Leader:  int32(bcf.lid),
								K:       int32(bcf.sid),
								Txcount: int32(len(valbuf[0].Value) / 250),
							},
						}
						backBlockByte, _ := proto.Marshal(backBlock)
						bcf.output <- backBlockByte
						//**11 bcf.log.Println("generate an output of height:", roundinfobuf[0].height)
						valbuf = valbuf[1:]

						roundinfobuf = roundinfobuf[1:]

					} else {
						//can't generate an output now
						break
					}
				} else {
					//no enough val or roundinfo
					break
				}
			}

		case roundinfo := <-bcf.roundinfoCH:
			//**11 bcf.log.Println("handle roundinfo of round in handle_val", roundinfo.height)
			roundinfobuf = append(roundinfobuf, roundinfo)
			lastheight = roundinfo.height
			if len(valbuf) > 0 {
				for i := 0; i < len(valbuf); i++ {
					if valbuf[i].Round == roundinfo.height+1 {
						hash := sha256.New()
						hash.Write(valbuf[i].Value)
						value_hash := hash.Sum(nil)
						go bcf.send_echo(int(roundinfo.height+1), value_hash)
					} else if valbuf[i].Round > roundinfo.height+1 {
						break
					}
				}
			}

			for {
				if len(roundinfobuf) > 0 && len(valbuf) > 0 {
					if valbuf[0].Round == roundinfobuf[0].height {
						hash := sha256.New()
						hash.Write(valbuf[0].Value)
						value_hash := hash.Sum(nil)
						if !bytes.Equal(value_hash, roundinfobuf[0].valuehash) {
							bcf.log.Fatalln("wrong leader 2")
						}
						//get a val we need,generate output and remove val and roundinfo
						backBlock := &pb.BCBlock{
							RawBC: &pb.RawBC{
								Height:  int32(roundinfobuf[0].height),
								Leader:  int32(bcf.lid),
								K:       int32(bcf.sid),
								Txcount: int32(len(valbuf[0].Value) / 250),
							},
						}
						backBlockByte, _ := proto.Marshal(backBlock)
						bcf.output <- backBlockByte
						//**11 bcf.log.Println("generate an output of height:", roundinfobuf[0].height)

						valbuf = valbuf[1:]
						roundinfobuf = roundinfobuf[1:]

					} else {
						//can't generate an output now
						break
					}
				} else {
					//no enough val or roundinfo
					break
				}
			}

		}

	}
}

func (bcf *BC_f) handle_echo(height int, readysignal chan []byte, oldbuf chan pb.WRBCMsg, futurebuf chan pb.WRBCMsg, closeCH chan bool) {
	//**11 bcf.log.Println("inside handle_echo")
	echomap := make(map[string]int)
	echocount := 0
	for {

		var msg pb.WRBCMsg
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
			//**11 bcf.log.Println("handle echo msg of round", height, "from", msg.ID)
			echocount++
			hashstring := string(msg.Content)
			_, ok := echomap[string(hashstring)]
			if ok {
				echomap[string(hashstring)]++
				if echomap[string(hashstring)] == bcf.threshold*2+1 {
					readysignal <- msg.Content
					return
				}
			} else {
				echomap[string(hashstring)] = 1
			}

		} else if msg.Round > int32(height) {
			select {
			case <-closeCH:

			default:
				SafeSend(futurebuf, msg)

			}

		}

		if echocount == bcf.num {
			return
		}

	}

}

func (bcf *BC_f) handle_ready(height int, readysignal chan []byte, outputsignal chan []byte, oldbuf chan pb.WRBCMsg, futurebuf chan pb.WRBCMsg, closeCH chan bool) {

	//**11 bcf.log.Println("inside handle_ready")
	readymap := make(map[string]int)
	for {
		var msg pb.WRBCMsg
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
			//**11 bcf.log.Println("handle ready msg of round", height, "from", msg.ID)
			hashstring := string(msg.Content)
			_, ok := readymap[hashstring]
			if ok {
				readymap[hashstring]++
			} else {
				readymap[hashstring] = 1
			}

			//count times receiving with the same root, if == f+1, send ready msg
			if readymap[hashstring] == bcf.threshold+1 {
				readysignal <- msg.Content
				//**11 bcf.log.Println("ready to send ready in ready")
			}

			//if ==2f+1, kill this process and wait for n-f response echo msg
			if readymap[hashstring] == bcf.threshold*2+1 {
				outputsignal <- msg.Content
				//**11 bcf.log.Println("ready to output")
				return
			}
		}

	}
}

func (bcf *BC_f) send_ready(height int, hash []byte) {
	//**11 bcf.log.Println("send ready msg of height ", height)
	for i := 0; i < bcf.num; i++ {

		if i+1 == bcf.nid {
			bcf.readyCH <- pb.WRBCMsg{
				ID:      int32(bcf.nid),
				Leader:  int32(bcf.lid),
				K:       int32(bcf.sid),
				Round:   int32(height),
				Type:    1,
				Content: hash,
			}
		} else {
			msg := pb.WRBCMsg{
				ID:      int32(bcf.nid),
				Leader:  int32(bcf.lid),
				K:       int32(bcf.sid),
				Round:   int32(height),
				Type:    1,
				Content: hash,
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
	//**11 bcf.log.Println("send ready msg of height", height, "done")
}

func (bcf *BC_f) send_echo(height int, hash []byte) {
	msg := pb.WRBCMsg{
		ID:      int32(bcf.nid),
		Leader:  int32(bcf.lid),
		K:       int32(bcf.sid),
		Round:   int32(height),
		Type:    2,
		Content: hash,
	}
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcf.log.Fatalln(err)
	}
	//**11 bcf.log.Println("send echo msg of height", height)

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

	//**11 bcf.log.Println("send echo msg of height", height, "done")
}

/*func (bcf BC_f) send_callhelp(height int, hash []byte) {
	bcf.log.Println("send callhelp msg of height", height)
	bcf.missMap.Lock.Lock()
	bcf.missMap.Missheights[height] = hash
	bcf.missMap.Lock.Unlock()
	msg := pb.WRBCMsg{
		ID:      int32(bcf.nid),
		Leader:  int32(bcf.lid),
		K:       int32(bcf.sid),
		Round:   int32(height),
		Type:    3,
		Content: hash,
	}
	//bcf.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcf.log.Fatalln(err)
	}

	for i := 0; i < bcf.threshold*2+1; i++ {
		if i+1 != bcf.nid {
			sendmsg := SendMsg{
				ID:      i + 1,
				Type:    0,
				Content: msgbyte,
			}
			bcf.msgOut <- sendmsg
		}

	}
	bcf.log.Println("send callhelp msg of height", height, "done")
}*/

/*func (bcf BC_f) send_help(height int, id int, value []byte) {
	bcf.log.Println("send help msg of height", height)
	msg := pb.WRBCMsg{
		ID:      int32(bcf.nid),
		Leader:  int32(bcf.lid),
		K:       int32(bcf.sid),
		Round:   int32(height),
		Type:    4,
		Content: value,
	}
	//bcf.log.Println(msg)
	msgbyte, err := proto.Marshal(&msg)
	if err != nil {
		bcf.log.Fatalln(err)
	}

	sendmsg := SendMsg{
		ID:      id,
		Type:    0,
		Content: msgbyte,
	}
	bcf.msgOut <- sendmsg


	bcf.log.Println("send help msg of height", height, "done")
}*/

/*func (bcf BC_f) handle_callhelp() {
	for {
		callhelpmsg := <-bcf.callhelpCH
		if callhelpmsg.Round < int32(bcf.height) {
			_, ok := bcf.missMap.Missheights[int(callhelpmsg.Round)]
			if !ok {
				bcf.log.Println("handle callhelp msg of round", callhelpmsg.Round, "from", callhelpmsg.ID)
				value, err := bcf.db.Get(IntToBytes(int(callhelpmsg.Round)))
				if err != nil {
					bcf.log.Panicln(err)
				}
				if value != nil {
					//generate help msg
					bcf.send_help(int(callhelpmsg.Round), int(callhelpmsg.ID), value)
				}
			}
		}
	}
}*/

/*func (bcf BC_f) handle_help() {
	for {
		helpmsg := <-bcf.helpCH
		bcf.log.Println("handle help msg of round", helpmsg.Round, "from", helpmsg.ID)
		bcf.missMap.Lock.Lock()
		misshash, ok := bcf.missMap.Missheights[int(helpmsg.Round)]

		if ok {
			value := helpmsg.Content
			hash := sha256.New()
			hash.Write(value)
			valuehash := hash.Sum(nil)
			if bytes.Equal(valuehash, misshash) {
				bcf.db.Put(IntToBytes(int(helpmsg.Round)), value)
				delete(bcf.missMap.Missheights, int(helpmsg.Round))
			}
		}
		bcf.missMap.Lock.Unlock()

	}
}*/

func routinechannel(old chan pb.WRBCMsg, new chan pb.WRBCMsg, height int) {
	for oldmsg := range old {
		if oldmsg.Round > int32(height) {
			new <- oldmsg
		}
	}
}

func SafeSend(ch chan pb.WRBCMsg, value pb.WRBCMsg) (closed bool) {
	defer func() {
		if recover() != nil {
			closed = true
		}
	}()

	ch <- value  // panic if ch is closed
	return false // <=> closed = false; return
}

