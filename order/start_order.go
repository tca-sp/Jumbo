package main

import (
	"bufio"
	"bytes"
	cy "dumbo_fabric/crypto/signature"
	bls "dumbo_fabric/crypto/signature/bls"
	ec "dumbo_fabric/crypto/signature/ecdsa"
	schnorr "dumbo_fabric/crypto/signature/schnorr"
	aggregate "dumbo_fabric/crypto/signature/schnorr_aggregate"
	"dumbo_fabric/database/leveldb"
	"dumbo_fabric/network"
	mvba "dumbo_fabric/order/smvba"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/golang/protobuf/proto"

	mapset "github.com/deckarep/golang-set"
)

const mspid = "SampleOrg"

var sleeptime int

var local bool

type order_m struct {
	Order_m        string `yaml:"Order_m"`
	Node_num       int    `yaml:"Node_num"`
	K              int    `yaml:"K"`
	SignatureType  string `yaml:"SignatureType"`
	FBHeight       int
	Round          int
	ID             int
	rcvBroadcastCH chan []byte        //buff msg rcv from all bc
	rcvOrderCH     chan []byte        //buff msg rcv from all order
	sendIntputCH   chan []byte        //share channel: send input to my order
	rcvOutputCH    chan pb.HighProofs //share channel: rcv output from my order
	smvbaCH        chan []byte        //share channel: proto msgs from other orders
	protoMsgOut    chan pb.SendMsg    //share channel: proto msgs to other orders
	callhelpCH     chan []byte
	helpCH         chan []byte
	dumbomvbaCH    chan []byte //share channel: dumbomvba msgs from other orders
	baCH           chan []byte //share channel: ba msgs from other orders
	mRBCCH         chan []byte //share channel: rbcwithfinish msgs from other orders
	msgOutCH       chan pb.SendMsg
	conMsgBuff     []chan pb.SendMsg
	lastCommit     [][]int
	heights        [][]pb.BCBlock //latest bcblocks
	oldBCBlocks    Oldblocks      //bcblocks buffer
	hs             [][]int32      //index of latest bcblocks
	ready          bool
	//cutBlockCH     chan cut
	prev_hash        []byte
	tmpContent       chan []byte
	ClientIPPath     string `yaml:"ClientIPPath"`
	OrderIPPath      string `yaml:"OrderIPPath"`
	SkPath           string `yaml:"SkPath"`
	PkPath           string `yaml:"PkPath"`
	sigmeta          cy.Signature
	ips              []string
	orderCons        []net.Conn
	ledger           [][][][]pb.BCBlock
	feedbackCH       chan pb.BCBlock
	DBPath           string `yaml:"DBPath"`
	db               leveldb.DB
	help2bcCH        chan pb.BCBlock
	callhelpbuffer   callhelpbuffer
	Testmode         bool `yaml:"Testmode"`
	IsControlSpeed   bool `yaml:"IsControlSpeed"`
	IsControlLatency bool `yaml:"IsControlLatency"`
	OrderNetSpeed    int  `yaml:"OrderNetSpeed"`
	OrderNetLatency  int  `yaml:"OrderNetLatency"`
	net              network.Network
	UsingDumboMVBA   bool   `yaml:"UsingDumboMVBA"`
	BroadcastType    string `yaml:"BroadcastType"`
	MVBAType         string `yaml:"MVBAType"`
	Sleeptime        int    `yaml:"Sleeptime"`
	BatchSize        int    `yaml:"BatchSize"`
	CrashMVBA        int    `yaml:"CrashMVBA"`
	ByzFairness      int    `yaml:"ByzFairness"`
}

func main() {
	local = false
	var idf = flag.Int("id", 0, "-id")
	flag.Parse()
	id := *idf

	//read node.yaml
	newOrder_m := &order_m{}
	gopath := os.Getenv("GOPATH")
	readBytes, err := ioutil.ReadFile(gopath + "/src/dumbo_fabric/config/node.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(readBytes, newOrder_m)
	if err != nil {
		panic(err)
	}

	sleeptime = newOrder_m.Sleeptime
	//hash_byte := []byte{238, 156, 103, 173, 69, 102, 24, 158, 26, 112, 216, 79, 42, 75, 226, 244, 120, 227, 150, 70, 248, 16, 184, 212, 36, 147, 209, 180, 8, 64, 155, 199}
	//newOrder_m.prev_hash = hash_byte

	newOrder_m.FBHeight = 0
	newOrder_m.ID = id
	newOrder_m.rcvBroadcastCH = make(chan []byte, 3000)
	newOrder_m.rcvOrderCH = make(chan []byte, 10000)
	newOrder_m.sendIntputCH = make(chan []byte, 3000)
	newOrder_m.rcvOutputCH = make(chan pb.HighProofs, 2)
	newOrder_m.smvbaCH = make(chan []byte, 3000)
	newOrder_m.protoMsgOut = make(chan pb.SendMsg, 10000)
	newOrder_m.callhelpCH = make(chan []byte, 3000)
	newOrder_m.helpCH = make(chan []byte, 3000)
	newOrder_m.dumbomvbaCH = make(chan []byte, 3000)
	newOrder_m.baCH = make(chan []byte, 10000)
	newOrder_m.mRBCCH = make(chan []byte, 30000)
	newOrder_m.msgOutCH = make(chan pb.SendMsg, 10000)
	newOrder_m.conMsgBuff = make([]chan pb.SendMsg, newOrder_m.Node_num)
	for i := 0; i < newOrder_m.Node_num; i++ {
		newOrder_m.conMsgBuff[i] = make(chan pb.SendMsg, 3000)
	}
	
	newOrder_m.heights = make([][]pb.BCBlock, newOrder_m.Node_num)
	//newOrder_m.cutBlockCH = make(chan cut, 100)
	newOrder_m.oldBCBlocks.blocks = make([][][]pb.BCBlock, newOrder_m.Node_num)
	//newOrder_m.oldBCBlocks.timestamps = make([][][]int64, newOrder_m.Node_num)
	newOrder_m.tmpContent = make(chan []byte, 3000)
	newOrder_m.orderCons = make([]net.Conn, newOrder_m.Node_num)
	newOrder_m.feedbackCH = make(chan pb.BCBlock, 3000)
	newOrder_m.help2bcCH = make(chan pb.BCBlock, 3000)
	newOrder_m.callhelpbuffer = callhelpbuffer{&sync.Mutex{}, make(map[key]mapset.Set[int32], newOrder_m.Node_num)}
	var lock sync.Mutex
	newOrder_m.oldBCBlocks.lock = &lock
	newOrder_m.net = network.New(newOrder_m.IsControlSpeed, newOrder_m.OrderNetSpeed, newOrder_m.IsControlLatency, newOrder_m.OrderNetLatency)
	newOrder_m.net.Init()
	for i := 0; i < newOrder_m.Node_num; i++ {
		newOrder_m.heights[i] = make([]pb.BCBlock, newOrder_m.K)
		newOrder_m.oldBCBlocks.blocks[i] = make([][]pb.BCBlock, newOrder_m.K)
		//newOrder_m.oldBCBlocks.timestamps[i] = make([][]int64, newOrder_m.K)
		for j := 0; j < newOrder_m.K; j++ {
			newOrder_m.heights[i][j] = pb.BCBlock{
				RawBC: &pb.RawBC{
					Height: -1,
				},
			}
		}

	}
	newOrder_m.hs = make([][]int32, newOrder_m.Node_num)
	for i := 0; i < newOrder_m.Node_num; i++ {
		newOrder_m.hs[i] = make([]int32, newOrder_m.K)
		for j := 0; j < newOrder_m.K; j++ {
			newOrder_m.hs[i][j] = -1
		}
	}
	newOrder_m.lastCommit = make([][]int, newOrder_m.Node_num)
	for i := 0; i < newOrder_m.Node_num; i++ {
		newOrder_m.lastCommit[i] = make([]int, newOrder_m.K)
		for j := 0; j < newOrder_m.K; j++ {
			newOrder_m.lastCommit[i][j] = -1
		}
	}
	newOrder_m.ready = true

	//read ip
	orderipmPath := fmt.Sprintf("%s%sipm.txt", gopath, newOrder_m.OrderIPPath)
	fi, err := os.Open(orderipmPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)
	var ordermIP string
	a, _, c := br.ReadLine()
	if c == io.EOF {
		panic("missing order_m ip")
	}
	if local {
		ordermIP = string(a) + fmt.Sprintf(":%d", 11000+id)
	} else {
		ordermIP = string(a)
	}

	fi.Close()

	//read order ips
	ipPath := fmt.Sprintf("%s%sip.txt", gopath, newOrder_m.OrderIPPath)
	fi, err = os.Open(ipPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br = bufio.NewReader(fi)
	ips := make([]string, newOrder_m.Node_num)
	var orderip string
	for i := 0; i < newOrder_m.Node_num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}

		if local {
			ips[i] = string(a) + fmt.Sprintf(":%d", 12000+id)
			if i == id-1 {
				orderip = string(a) + fmt.Sprintf(":%d", 12000+id)
			}
		} else {
			ips[i] = string(a) + fmt.Sprintf(":%d", 12000)
			if i == id-1 {
				orderip = string(a) + fmt.Sprintf(":%d", 12000)
			}
		}

	}
	fi.Close()
	newOrder_m.ips = ips

	//init signature
	var signature cy.Signature
	switch newOrder_m.SignatureType {
	case "ecdsa":
		sig := ec.NewSignature()
		signature = &sig
	case "schnorr":
		sig := schnorr.NewSignature()
		signature = &sig
	case "schnorr_aggregate":
		sig := aggregate.NewSignature()
		signature = &sig
	case "bls":
		sig := bls.NewSignature()
		signature = &sig
	default:
		panic("wrong signature type")
	}
	signature.Init(gopath+newOrder_m.PkPath, newOrder_m.Node_num, id)
	newOrder_m.sigmeta = signature

	//init database
	if !newOrder_m.Testmode {
		dbpath := fmt.Sprintf("%s%sorder/%d", gopath, newOrder_m.DBPath, id)
		//dbpath := gopath + "/src/super_fabric/sample_sf/database/leveldb/db"
		db := leveldb.CreateDB(dbpath)
		err = os.Mkdir(dbpath, 0750)
		if err != nil && !os.IsExist(err) {
			panic(err)
		}
		db.Open()
		defer db.Close()
		newOrder_m.db = *db
	}

	//receive broadcast heights from my broadcasters
	go network.Listen_msg(ordermIP, newOrder_m.rcvBroadcastCH, true)
	//receive msg from other orderes
	go network.Listen_msg(orderip, newOrder_m.rcvOrderCH, true)
	//deal with msg from other orders
	go newOrder_m.handle_rcvOrderCH()
	//deal with broadcast heights from my broadcasters
	go newOrder_m.handle_rcvBroadcastCH()
	//deal with outputs from MVBA
	go newOrder_m.handle_rcvOutputCH()
	//go newOrder_m.handle_feedbackCH()
	go newOrder_m.handle_protomsgOutCH()
	go newOrder_m.handle_msgOutCH()
	//go newOrder_m.handle_callhelpCH()
	//go newOrder_m.handle_helpCH()
	for i := 0; i < newOrder_m.Node_num; i++ {
		if i != id-1 {
			go newOrder_m.handle_orderCon(i + 1)
		}

	}

	//init new order and start it

	newOrder := mvba.NewOrder(id, newOrder_m.Node_num, newOrder_m.K, orderip, ips, newOrder_m.sigmeta, newOrder_m.sendIntputCH, newOrder_m.rcvOutputCH, check_inputs, check_input_RBC, check_inputs_QCagg, newOrder_m.smvbaCH, newOrder_m.protoMsgOut, newOrder_m.UsingDumboMVBA, newOrder_m.dumbomvbaCH, newOrder_m.BroadcastType, newOrder_m.MVBAType, newOrder_m.baCH, newOrder_m.mRBCCH, newOrder_m.hs, newOrder_m.oldBCBlocks.blocks,newOrder_m.ByzFairness)
	newOrder.Start()

}

//receive blocks from broadcaster
func (od_m *order_m) handle_rcvBroadcastCH() {
	if od_m.ID > od_m.Node_num-od_m.CrashMVBA {
		for {
			<-od_m.rcvBroadcastCH
		}
	}
	for {
		//update world status
		var blk pb.BCBlock
		select {
		case msg := <-od_m.rcvBroadcastCH:
			err := proto.Unmarshal(msg, &blk)
			if err != nil {
				panic(err)
			}
		case blk = <-od_m.help2bcCH: //coming from others help
		}

		//fmt.Println("check if bcblock right ", len(blk.Payload))
		sid := blk.RawBC.K
		lid := blk.RawBC.Leader
		//fmt.Println("get a msg from bc in handle_rcvBroadcastCH ", sid, lid)
		//fmt.Println("sign id ", blk.Sign.ID)
		//shardID := blk.Sign.ID
		od_m.update_bcBlocks(blk)

		//od_m.callhelpbuffer.lock.Lock()
		if !od_m.Testmode {
			//store block in database
			key := append(IntToBytes(int(blk.RawBC.Height)), IntToBytes(int(lid))...)
			key = append(key, IntToBytes(int(sid))...)
			blkbyte, err := proto.Marshal(&blk)
			if err != nil {
				panic(err)
			}
			od_m.db.Put(key, blkbyte)
		}

		//check if can help others now
		//od_m.check_callhelpbuff(lid, sid, blk.RawBC.Height, key, blkbyte)
		//od_m.callhelpbuffer.lock.Unlock()
		//if it's time to generate a new input to order
		if od_m.ready {
			od_m.check_send()
		}
	}

}

//check if some other node called help for this bcblock
func (od_m *order_m) check_callhelpbuff(lid int32, sid int32, height int32, Key []byte, blkbyte []byte) {
	value, ok := od_m.callhelpbuffer.missblocks[key{lid, sid, height}]
	if !ok {
		//no one has called help for this bcblock
		return
	} else {
		//i can help now, generate help msg
		helpmsg := pb.OrderMsg{
			Type:    3,
			Content: blkbyte,
		}
		helpbyte, err := proto.Marshal(&helpmsg)
		if err != nil {
			panic(err)
		}

		for id := range value.Iter() {
			sendmsg := pb.SendMsg{
				ID:   int(id),
				Type: 3,
				Msg:  helpbyte,
				//key:     Key,
			}
			od_m.msgOutCH <- sendmsg
		}
		od_m.callhelpbuffer.remove(lid, sid, height)
	}
}

//call when receive a bcblock
func (od_m *order_m) update_bcBlocks(bcblock pb.BCBlock) {
	od_m.oldBCBlocks.lock.Lock()
	defer od_m.oldBCBlocks.lock.Unlock()
	lid := bcblock.RawBC.Leader
	sid := bcblock.RawBC.K
	if bcblock.RawBC.Height > od_m.heights[lid-1][sid-1].RawBC.Height {
		od_m.heights[lid-1][sid-1] = bcblock
		od_m.hs[lid-1][sid-1] = bcblock.RawBC.Height
	}
	//fmt.Println("update hs:", od_m.hs)
	od_m.add_bcBlock(bcblock)
}

//to be done,caculate timestamp
func (od_m *order_m) add_bcBlock(bcblock pb.BCBlock) {
	lid := bcblock.RawBC.Leader - 1
	sid := bcblock.RawBC.K - 1
	var tmpold []pb.BCBlock
	flag := false
	if len(od_m.oldBCBlocks.blocks[lid][sid]) == 0 {
		od_m.oldBCBlocks.blocks[lid][sid] = append(od_m.oldBCBlocks.blocks[lid][sid], bcblock)
	} else {
		for i := 0; i < len(od_m.oldBCBlocks.blocks[lid][sid]); i++ {
			if od_m.oldBCBlocks.blocks[lid][sid][i].RawBC.Height > bcblock.RawBC.Height {
				if !flag {
					tmpold = append(tmpold, bcblock)
					flag = true
				}
				tmpold = append(tmpold, od_m.oldBCBlocks.blocks[lid][sid][i])
			} else if od_m.oldBCBlocks.blocks[lid][sid][i].RawBC.Height == bcblock.RawBC.Height {
				tmpold = append(tmpold, od_m.oldBCBlocks.blocks[lid][sid][i])
				flag = true
			} else {
				tmpold = append(tmpold, od_m.oldBCBlocks.blocks[lid][sid][i])
				if i == len(od_m.oldBCBlocks.blocks[lid][sid])-1 {
					tmpold = append(tmpold, bcblock)
				}
			}
		}
		od_m.oldBCBlocks.blocks[lid][sid] = tmpold
	}
}

func (od_m *order_m) handle_rcvOrderCH() {
	for {
		msg := <-od_m.rcvOrderCH
		ordermsg := pb.OrderMsg{}
		err := proto.Unmarshal(msg, &ordermsg)
		if err != nil {
			panic(err)
		}
		switch ordermsg.Type {
		case 1: //smvba msg
			//fmt.Println("get a proto msg in handle_rcvOrderCH")
			od_m.smvbaCH <- ordermsg.Content
		case 2: //get a callhelp msg
			fmt.Println("get a callhelp msg")
			od_m.callhelpCH <- ordermsg.Content
		case 3: //get a help msg
			fmt.Println("get a help msg")
			od_m.helpCH <- ordermsg.Content
		case 4: //get a dumbomvba msg
			od_m.dumbomvbaCH <- ordermsg.Content
		case 5: //bamsg for fin
			select {
			case od_m.baCH <- ordermsg.Content:
			default:
				fmt.Println("ba buffer full")
				od_m.baCH <- ordermsg.Content
			}
		//od_m.baCH <- ordermsg.Content
		case 6: //modified rbc msg for fin
			select {
			case od_m.mRBCCH <- ordermsg.Content:
			default:
				fmt.Println("mRBCCH full")
				od_m.mRBCCH <- ordermsg.Content
			}	
		//od_m.mRBCCH <- ordermsg.Content
		default:
			panic("wrong type order msg")
		}

	}
}

//handle callhelp msgs, which contains locations of sender's miss blocks
/*func (od_m *order_m) handle_callhelpCH() {
	for {
		msgbyte := <-od_m.callhelpCH
		callhelp := pb.CallHelpOrder{}
		err := proto.Unmarshal(msgbyte, &callhelp)
		if err != nil {
			panic(err)
		}

		for _, misblock := range callhelp.MissBlocks {
			lid := misblock.Lid
			sid := misblock.Sid
			for _, height := range misblock.MissHeights {
				key := append(IntToBytes(int(height)), IntToBytes(int(lid))...)
				key = append(key, IntToBytes(int(sid))...)
				//od_m.callhelpbuffer.lock.Lock()
				blockbyte, err := od_m.db.Get(key)
				if err != nil {
					panic(err)
				}
				if blockbyte == nil {
					//can't help now
					//buffer index of missblocks until i can help
					//od_m.callhelpbuffer.put(lid, sid, height, callhelp.ID)

				} else {
					//i can help now, generate help msg
					sendmsg := pb.SendMsg{
						ID:      int(callhelp.ID),
						Type: 3,
						Msg: blockbyte,
						//key:     key,
					}
					od_m.msgOutCH <- sendmsg

				}
				//od_m.callhelpbuffer.lock.Unlock()
			}
		}

	}
}*/

//handle hlep msgs, which contains blocks I missed
//func (od_m *order_m) handle_helpCH() {
//	for {
//		msg := <-od_m.helpCH
//		bcblock := pb.BCBlock{}
//		err := proto.Unmarshal(msg, &bcblock)
//		if err != nil {
//			panic(err)
//		}
//		hash := sha256.New()
//		hashMsg, _ := proto.Marshal(bcblock.RawBC)
//		hash.Write(hashMsg)
//		blkID := hash.Sum(nil)[:]
//
//		signbyte := append(blkID, IntToBytes(int(bcblock.RawBC.//Height))...)
//		signbyte = append(signbyte, IntToBytes(int(bcblock.RawBC.//Leader))...)
//		signbyte = append(signbyte, IntToBytes(int(bcblock.RawBC.//K))...)
//
//		//check := true
//		//fmt.Println("wrong combine signs of height ", bcblock.RawBC.//Height)
//		check := od_m.sigmeta.BatchSignatureVerify(bcblock.BatchSigns.//Signs, signbyte, bcblock.BatchSigns.Mems)
//		/*for _, sign := range bcblock.Signs {
//			res := od_m.sigmeta.Verify(int(sign.ID-1), sign.Content, //signbyte)
//			if !res {
//				check = false
//				break
//			}
//		}*/
//		if !check {
//			err := fmt.Sprintln("wrong combine signs of height ", //bcblock.RawBC.Height)
//			panic(err)
//		}
//
//		od_m.help2bcCH <- bcblock
//
//	}
//
//}

//receive an output from MVBA
func (od_m *order_m) handle_rcvOutputCH() {
	var alllatency time.Duration
	var alllatencyblockcount int
	var allblockcount int
	isfirsttime := true
	var starttime time.Time
	var elapsed time.Duration
	for {
		hps := <-od_m.rcvOutputCH
		timetmp := time.Now()
		fmt.Println(timetmp, "finish a round of mvba")
		if isfirsttime {
			starttime = timetmp
		} else {
			elapsed = timetmp.Sub(starttime)
		}
		/*hps := &pb.HighProofs{}
		err := proto.Unmarshal(msg.RawMsg.Values, hps)
		if err != nil {
			panic(err)
		}*/

		//fmt.Println("*******highproofs", hps)

		//update world status
		tmpHeights := make([][]int, od_m.Node_num)
		totalheight := 0
		for i := 0; i < od_m.Node_num; i++ {
			tmpHeights[i] = make([]int, od_m.K)
			for j := 0; j < od_m.K; j++ {
				if !reflect.DeepEqual(hps.HPs[j*od_m.Node_num+i], pb.HighProof{}) {
					tmpHeights[i][j] = od_m.lastCommit[i][j]
					od_m.lastCommit[i][j] = int(hps.HPs[j*od_m.Node_num+i].RawBC.Height)
					totalheight = totalheight + int(hps.HPs[j*od_m.Node_num+i].RawBC.Height)
				}
			}
		}
		fmt.Println("total height：", totalheight)
		fmt.Println("*****lastcommit", tmpHeights, " ", od_m.lastCommit)
		fmt.Println("len of send buff", len(od_m.protoMsgOut))
		fmt.Println("len of receive buff", len(od_m.smvbaCH), len(od_m.dumbomvbaCH), len(od_m.baCH), len(od_m.mRBCCH))
		//to be done: cut block
		//od_m.cutBlockCH <- cut{tmpHeights, od_m.lastCommit}
		roundlatency, latencyblockcount, roundblockcount := od_m.cutBlock(tmpHeights, od_m.lastCommit, hps)
		//alllatency += roundlatency
		//alllatencyblockcount += latencyblockcount
		//allblockcount += roundblockcount
		if isfirsttime {
			isfirsttime = false
		} else {
			allblockcount += roundblockcount
			alllatency += roundlatency
			alllatencyblockcount += latencyblockcount
			tps := float64(allblockcount) / (float64(elapsed) / float64(time.Second))
			fmt.Println("all tps:", tps)
		}
		if alllatencyblockcount != 0 {
			fmt.Println("all latency:", alllatency/time.Duration(alllatencyblockcount))
		}

		//start generate next input

		od_m.check_send()

	}
}

func (od_m *order_m) handle_protomsgOutCH() {
	for {
		msg := <-od_m.protoMsgOut
		//fmt.Println("get a protocol msg")

		od_m.msgOutCH <- msg
	}

}

func (od_m *order_m) handle_msgOutCH() {
	for {
		msg := <-od_m.msgOutCH
		//fmt.Println("get a msg inside handle_msgOutCH")
		if msg.ID > od_m.Node_num || msg.ID <= 0 {
			panic("wrong msg ID")
		}
		select {
		case od_m.conMsgBuff[msg.ID-1] <- msg:
		default:
			fmt.Println("send buffer full ", msg.ID)
			od_m.conMsgBuff[msg.ID-1] <- msg
		}

	}
}

//to be done: caculate latency
func (od_m *order_m) cutBlock(old [][]int, new [][]int, hps pb.HighProofs) (time.Duration, int, int) {

	timeend := time.Now()
	differ := make([][]int, od_m.Node_num)
	for i := 0; i < od_m.Node_num; i++ {
		differ[i] = make([]int, od_m.K)
	}

	for i := 0; i < od_m.Node_num; i++ {
		for j := 0; j < od_m.K; j++ {
			differ[i][j] = new[i][j] - old[i][j]
		}

	}
	callhelp := pb.CallHelpOrder{
		Round: int32(od_m.Round),
		ID:    int32(od_m.ID),
	}
	callinghlp := false //true means we have called help, no need to call help again
	firsttime := true
	for {
		flag := true
		for i := 0; i < od_m.Node_num; i++ {
			for j := 0; j < od_m.K; j++ {
				if len(od_m.oldBCBlocks.blocks[i][j]) < differ[i][j] {
					flag = false
					/*if !callinghlp {
						missblock := pb.MissBlock{
							Lid: int32(i + 1),
							Sid: int32(j + 1),
						}
						k := int32(0)
						if len(od_m.oldBCBlocks.blocks[i][j]) > 0 {
							k = od_m.oldBCBlocks.blocks[i][j][len(od_m.oldBCBlocks.blocks[i][j])-1].RawBC.Height + 1
						}
						for ; int(k) <= od_m.lastCommit[i][j]; k++ {
							missblock.MissHeights = append(missblock.MissHeights, k)
						}

						callhelp.MissBlocks = append(callhelp.MissBlocks, &missblock)
					}*/
					if firsttime {
						fmt.Println(i, "|", j, "|", len(od_m.oldBCBlocks.blocks[i][j]), "|", differ[i][j])
					}

				}
			}
		}
		firsttime = false
		if flag {
			break
		} else {
			//call help or just waiting?
			//waiting
			//time.Sleep(time.Duration(50) * time.Millisecond)

			//working
			//call help
			if !od_m.Testmode {
				if !callinghlp {
					callhelpbyte, err := proto.Marshal(&callhelp)
					if err != nil {
						panic(err)
					}
					rand.Seed(time.Now().UnixNano())
					id := rand.Intn(od_m.Node_num)
					for i := 0; i < od_m.Node_num/3*2+1; i++ {
						sendmsg := pb.SendMsg{
							ID:   (id+i)%od_m.Node_num + 1,
							Type: 2,
							Msg:  callhelpbyte,
						}
						od_m.msgOutCH <- sendmsg
					}
					callinghlp = true
				}
			}

			//waiting until receive all missing blocks
			time.Sleep(time.Duration(50) * time.Millisecond)
		}
	}

	//extract blocks
	fmt.Println("------------extract blocks")
	od_m.oldBCBlocks.lock.Lock()
	defer od_m.oldBCBlocks.lock.Unlock()
	//newblock := make([][][]pb.BCBlock, od_m.Node_num)
	var timecount time.Duration
	blockcount := 0 //caculate how many block have been commit this round
	latencyblockcount := 0
	blockbuffercount := 0
	count:=0
	for i := 0; i < od_m.Node_num; i++ {
		//newblock[i] = make([][]pb.BCBlock, od_m.K)
		for j := 0; j < od_m.K; j++ {

			//feedback to client
			//Q? send back block or txs?
			//newblock[i][j] = od_m.oldBCBlocks.blocks[i][j][:differ[i][j]]
			//send block back to client
			/*if i == 0 {
				for _, block := range newblock[0][j] {
					od_m.feedbackCH <- block
				}
			}*/
			for k := 0; k < differ[i][j]; k++ {
				if i == od_m.ID-1 {
					timestamp := od_m.oldBCBlocks.blocks[i][j][k].RawBC.Timestamp
					if timestamp != 0 {
						timetmp := time.Unix(0, timestamp)
						elapsed := timeend.Sub(timetmp)
						timecount += elapsed * time.Duration(od_m.oldBCBlocks.blocks[i][j][k].RawBC.Txcount)
						latencyblockcount += int(od_m.oldBCBlocks.blocks[i][j][k].RawBC.Txcount)
					}
				}
				//if od_m.BroadcastType == "WRBC" {
				//	blockcount += od_m.BatchSize
				//} else {
				blockcount += int(od_m.oldBCBlocks.blocks[i][j][k].RawBC.Txcount)
				//}
				count++
			}
			blockbuffercount += len(od_m.oldBCBlocks.blocks[i][j])
			od_m.oldBCBlocks.blocks[i][j] = od_m.oldBCBlocks.blocks[i][j][differ[i][j]:]
		}
	}
	fmt.Println("len of blockbuffer:", blockbuffercount)
	if latencyblockcount != 0 {
		latency := timecount / time.Duration(latencyblockcount)
		fmt.Println("round latency ", latency)
	}

	//od_m.ledger = append(od_m.ledger, newblock)
	fmt.Println("------------extract blocks done, average batch size:", blockcount/count)
	return timecount, latencyblockcount, blockcount

}

func (od_m *order_m) handle_orderCon(id int) {
	time.Sleep(time.Duration(sleeptime) * time.Second)
	for {
		con, err := network.Dial(od_m.ips[id-1])
		fmt.Println("dial:", od_m.ips[id-1])
		if err != nil {
			fmt.Println(err)
			time.Sleep(time.Duration(1) * time.Second)
		} else {
			fmt.Println("dial:", od_m.ips[id-1], "done")
			od_m.orderCons[id-1] = con
			od_m.handle_conMsg(id)
			con.Close()
		}

	}
}

//send msg to other orders
func (od_m *order_m) handle_conMsg(id int) {
	//protoMsg := make(chan sendmsg, 1)
	//recovMsg := make(chan []byte, 3000)
	//callhelpMsg := make(chan sendmsg, 3000)
	dropflag := false
	waitcount := 0
	if id > od_m.Node_num-od_m.CrashMVBA {
		for {
			<-od_m.conMsgBuff[id-1]
		}
	}

	if od_m.ID > od_m.Node_num-od_m.CrashMVBA {
		for {
			<-od_m.conMsgBuff[id-1]
		}
	}

	for {
		//get msg

		msg := <-od_m.conMsgBuff[id-1]

		//try send

		if !dropflag {
			ordermsg := pb.OrderMsg{
				Type:    int32(msg.Type),
				Content: msg.Msg,
			}
			msgbyte, err := proto.Marshal(&ordermsg)
			if err != nil {
				panic(err)
			}
			for {
				err := od_m.net.Send(od_m.orderCons[id-1], msgbyte)
				if err != nil {
					for {
						con, err := network.Dial(od_m.ips[id-1])
						fmt.Println("dial:", od_m.ips[id-1])
						if err != nil {
							time.Sleep(time.Duration(1) * time.Second)
							waitcount++
							if waitcount > 5 {
								dropflag = true
								break
							}
						} else {
							od_m.orderCons[id-1] = con
							break
						}
					}

				} else {
					break
				}
				if dropflag {
					break
				}

			}
		}
		/*err = network.Send(od_m.orderCons[id-1], msgbyte)
		//if fail, try reconnect and buffer msgs
		if err != nil {
			close := make(chan bool)
			//reconnect
			go od_m.handle_reconnect(id, close)
			//buffer msg
			od_m.handle_buffConMsg(id, close, msg, protoMsg, recovMsg, callhelpMsg)
		}*/
		//fmt.Println("done send a msg to ", id)
	}
}

//func (od_m *order_m) handle_buffConMsg(id int, close chan bool, initmsg sendmsg, protomsg chan sendmsg, //recovmsg chan []byte, callhelpmsg chan sendmsg) {
//	switch initmsg.msgtype {
//	case 1:
//		//to be done, only buff msg within two(or other constant) latest round
//		protomsg <- initmsg
//	case 2:
//		callhelpmsg <- initmsg
//	case 3:
//		recovmsg <- initmsg.key
//	default:
//		panic("wrong type msg")
//	}
//
//	for {
//		select {
//		case <-close:
//			return
//		default:
//		}
//		msg := <-od_m.conMsgBuff[id-1]
//		switch msg.msgtype {
//		case 1:
//			protomsg <- msg
//		case 2:
//			callhelpmsg <- msg
//		case 3:
//			recovmsg <- msg.key
//		default:
//			panic("wrong type msg")
//		}
//	}
//}

func (od_m *order_m) handle_reconnect(id int, close chan bool) {
	for {
		con, err := network.Dial(od_m.ips[id-1])
		fmt.Println("dial:", od_m.ips[id-1])
		if err != nil {
			panic(err)
		} else {
			od_m.orderCons[id-1] = con
			SafeClose(close)
		}

		time.Sleep(time.Duration(1) * time.Second)
	}

}

func (od_m *order_m) handle_feedbackCH() {
	gopath := os.Getenv("GOPATH")
	ClientIPPath := fmt.Sprintf("%s%sip.txt", gopath, od_m.ClientIPPath)
	fi, err := os.Open(ClientIPPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)
	var clientIP string
	for i := 0; i < od_m.Node_num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		if i == od_m.ID-1 {
			clientIP = string(a)
		}

	}
	fi.Close()

	var clientCon net.Conn

	block := <-od_m.feedbackCH
	blockBytes, err := proto.Marshal(&block)
	if err != nil {
		panic(err)
	}

	clientCon, err = network.Dial(clientIP)
	fmt.Println("dial:", clientIP)
	if err != nil {
		fmt.Println(od_m.ID, " ", err)
	}
	if od_m.ID == 1 {
		od_m.net.Send(clientCon, blockBytes)

		fmt.Println("done payback")
	}

	for {
		block := <-od_m.feedbackCH
		blockBytes, err := proto.Marshal(&block)
		if err != nil {
			panic(err)
		}
		if od_m.ID == 1 {
			od_m.net.Send(clientCon, blockBytes)

			fmt.Println("done payback")
		}
	}

}

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

func SafeClose(ch chan bool) {
	defer func() {
		if recover() != nil {
			// close(ch) panic occur
		}
	}()

	close(ch) // panic if ch is closed
}

//lid sid height id
func (chb callhelpbuffer) put(x int32, y int32, z int32, v int32) {
	if value, ok := chb.missblocks[key{x, y, z}]; ok {
		value.Add(v)
	} else {
		chb.missblocks[key{x, y, z}] = mapset.NewSet[int32]()
		chb.missblocks[key{x, y, z}].Add(v)
	}

}

//lid sid height
func (chb callhelpbuffer) remove(x int32, y int32, z int32) {
	delete(chb.missblocks, key{x, y, z})
}
