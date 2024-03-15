package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	//broadcast "dumbo_fabric/broadcaster/broadcast/rbc"
	//broadcast "dumbo_fabric/broadcaster/broadcast/cbc"
	cbc "dumbo_fabric/broadcaster/broadcast/cbc"
	rbc "dumbo_fabric/broadcaster/broadcast/rbc"
	wrbc "dumbo_fabric/broadcaster/broadcast/wrbc"
	cy "dumbo_fabric/crypto/signature"
	bls "dumbo_fabric/crypto/signature/bls"
	ec "dumbo_fabric/crypto/signature/ecdsa"
	schnorr "dumbo_fabric/crypto/signature/schnorr"
	aggregate "dumbo_fabric/crypto/signature/schnorr_aggregate"

	"dumbo_fabric/database/leveldb"
	"dumbo_fabric/network"

	pb "dumbo_fabric/struct"

	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"
)

var sleeptime int

type BC_m struct {
	Node_num              int    `yaml:"Node_num"`
	K                     int    `yaml:"K"`
	BatchSize             int    `yaml:"BatchSize"`
	PkPath                string `yaml:"PkPath"`
	SkPath                string `yaml:"SkPath"`
	BCIPPath              string `yaml:"BCIPPath"`
	OrderIPPath           string `yaml:"OrderIPPath"`
	TpIPPath              string `yaml:"TpIPPath"`
	DBPath                string `yaml:"DBPath"`
	SignatureType         string `yaml:"SignatureType"`
	Testmode              bool   `yaml:"Testmode"`
	IsControlSpeed        bool   `yaml:"IsControlSpeed"`
	IsControlLatency      bool   `yaml:"IsControlLatency"`
	BroadcasterNetSpeed   int    `yaml:"BroadcasterNetSpeed"`
	BroadcasterNetLatency int    `yaml:"BroadcasterNetLatency"`
	BroadcastType         string `yaml:"BroadcastType"`
	Sleeptime             int    `yaml:"Sleeptime"`
	IsControlBatch        bool   `yaml:"IsControlBatch"`
	
	CrashMVBA             int    `yaml:"CrashMVBA"`
}

type BC struct {
	nid       int
	lid       int
	sid       int
	node_num  int
	k         int
	BatchSize int
	sigmeta   cy.Signature
	ips       []string
	ordermIP  string
	txpoolIP  string
	txmsgIn   chan []byte //msg from txpool
	msg2order chan []byte //msg to orderm
	bcmsgIn   chan []byte //msg from other broadcast
	//bcmsgOut            chan broadcast.SendMsg //msg to other broadcast
	bcmsgOut_rbc  chan rbc.SendMsg
	bcmsgOut_cbc  chan cbc.SendMsg
	bcmsgOut_wrbc chan wrbc.SendMsg
	bc2fCon       []net.Conn
	//conMsgBuf           []chan broadcast.SendMsg
	conMsgBuf_rbc       []chan rbc.SendMsg
	conMsgBuf_cbc       []chan cbc.SendMsg
	conMsgBuf_wrbc      []chan wrbc.SendMsg
	orderCon            net.Conn
	signal2tpCH         chan []byte
	tpCon               net.Conn
	height              int
	DB                  *leveldb.DB
	Testmode            bool
	BroadcasterNetSpeed int
	net                 network.Network
	bctype              string
	IsControlBatch      bool
	CrashMVBA           int
}

func main() {
	var nidf = flag.Int("nid", 0, "-nid")
	var lidf = flag.Int("lid", 0, "-lid")
	var sidf = flag.Int("sid", 0, "-sid")
	flag.Parse()

	nid := *nidf
	lid := *lidf
	sid := *sidf

	//read config
	newBC_m := &BC_m{}
	gopath := os.Getenv("GOPATH")
	fmt.Println(gopath)
	readBytes, err := ioutil.ReadFile(gopath + "/src/dumbo_fabric/config/node.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(readBytes, newBC_m)
	if err != nil {
		panic(err)
	}

	sleeptime = newBC_m.Sleeptime

	//init signature
	var signature cy.Signature
	switch newBC_m.SignatureType {
	case "ecdsa":
		fmt.Println("use ecdsa")
		sig := ec.NewSignature()
		signature = &sig
	case "schnorr":
		fmt.Println("use schnorr")
		sig := schnorr.NewSignature()
		signature = &sig
	case "schnorr_aggregate":
		fmt.Println("use schnorr_aggregate")
		sig := aggregate.NewSignature()
		signature = &sig
	case "bls":
		fmt.Println("use schnorr_aggregate")
		sig := bls.NewSignature()
		signature = &sig
	default:
		panic("wrong signature type")
	}
	signature.Init(gopath+newBC_m.PkPath, newBC_m.Node_num, nid)

	//read IP
	ipPath := fmt.Sprintf("%s%sip.txt", gopath, newBC_m.BCIPPath)
	fi, err := os.Open(ipPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)
	ips := make([]string, newBC_m.Node_num)

	for i := 0; i < newBC_m.Node_num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		ips[i] = string(a) + fmt.Sprintf(":%d", 14000+sid*newBC_m.Node_num+lid)
	}
	fi.Close()

	orderipmPath := fmt.Sprintf("%s%sipm.txt", gopath, newBC_m.OrderIPPath)
	fi, err = os.Open(orderipmPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br = bufio.NewReader(fi)
	var ordermIP string
	a, _, c := br.ReadLine()
	if c == io.EOF {
		panic("missing order ip")
	}
	ordermIP = string(a)

	fi.Close()


	TpIPPath := fmt.Sprintf("%s%sipm.txt", gopath, newBC_m.TpIPPath)
	fi, err = os.Open(TpIPPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br = bufio.NewReader(fi)
	var tpIP string
	a, _, c = br.ReadLine()
	if c == io.EOF {
		panic("missing txpool ip")
	}
	tpIP = string(a)

	fi.Close()

	if newBC_m.BroadcastType == "CBC_QCagg" {
		newBC_m.BroadcastType = "CBC"
	}

	var newBC *BC
	if newBC_m.BroadcastType == "RBC" {
		newBC = &BC{
			nid:                 nid,
			lid:                 lid,
			sid:                 sid,
			node_num:            newBC_m.Node_num,
			k:                   newBC_m.K,
			BatchSize:           newBC_m.BatchSize,
			sigmeta:             signature,
			ips:                 ips,
			ordermIP:            ordermIP,
			txpoolIP:            tpIP,
			txmsgIn:             make(chan []byte, 2000),
			msg2order:           make(chan []byte, 500),
			bcmsgIn:             make(chan []byte, 500),
			bcmsgOut_rbc:        make(chan rbc.SendMsg, 500),
			bc2fCon:             make([]net.Conn, newBC_m.Node_num),
			height:              0,
			conMsgBuf_rbc:       make([]chan rbc.SendMsg, newBC_m.Node_num),
			signal2tpCH:         make(chan []byte, 2),
			Testmode:            newBC_m.Testmode,
			BroadcasterNetSpeed: newBC_m.BroadcasterNetSpeed,
			net:                 network.New(newBC_m.IsControlSpeed, newBC_m.BroadcasterNetSpeed, newBC_m.IsControlLatency, newBC_m.BroadcasterNetLatency),
			bctype:              newBC_m.BroadcastType,
			IsControlBatch:      newBC_m.IsControlBatch,
		}
	} else if newBC_m.BroadcastType == "CBC" {
		newBC = &BC{
			nid:                 nid,
			lid:                 lid,
			sid:                 sid,
			node_num:            newBC_m.Node_num,
			k:                   newBC_m.K,
			BatchSize:           newBC_m.BatchSize,
			sigmeta:             signature,
			ips:                 ips,
			ordermIP:            ordermIP,
			txpoolIP:            tpIP,
			txmsgIn:             make(chan []byte, 2),
			msg2order:           make(chan []byte, 500),
			bcmsgIn:             make(chan []byte, 500),
			bcmsgOut_cbc:        make(chan cbc.SendMsg, 500),
			bc2fCon:             make([]net.Conn, newBC_m.Node_num),
			height:              0,
			conMsgBuf_cbc:       make([]chan cbc.SendMsg, newBC_m.Node_num),
			signal2tpCH:         make(chan []byte, 2),
			Testmode:            newBC_m.Testmode,
			BroadcasterNetSpeed: newBC_m.BroadcasterNetSpeed,
			net:                 network.New(newBC_m.IsControlSpeed, newBC_m.BroadcasterNetSpeed, newBC_m.IsControlLatency, newBC_m.BroadcasterNetLatency),
			bctype:              newBC_m.BroadcastType,
			IsControlBatch:      newBC_m.IsControlBatch,
			
			CrashMVBA:           newBC_m.CrashMVBA,
		}
	} else if newBC_m.BroadcastType == "WRBC" {
		newBC = &BC{
			nid:            nid,
			lid:            lid,
			sid:            sid,
			node_num:       newBC_m.Node_num,
			k:              newBC_m.K,
			BatchSize:      newBC_m.BatchSize,
			sigmeta:        signature,
			ips:            ips,
			ordermIP:       ordermIP,
			txpoolIP:       tpIP,
			txmsgIn:        make(chan []byte, 2000),
			msg2order:      make(chan []byte, 500),
			bcmsgIn:        make(chan []byte, 500),
			bcmsgOut_wrbc:  make(chan wrbc.SendMsg, 500),
			bc2fCon:        make([]net.Conn, newBC_m.Node_num),
			height:         0,
			conMsgBuf_wrbc: make([]chan wrbc.SendMsg, newBC_m.Node_num),
			signal2tpCH:    make(chan []byte, 2),
			Testmode:       newBC_m.Testmode,
			net:            network.New(newBC_m.IsControlSpeed, newBC_m.BroadcasterNetSpeed, newBC_m.IsControlLatency, newBC_m.BroadcasterNetLatency),
			bctype:         newBC_m.BroadcastType,
			IsControlBatch: newBC_m.IsControlBatch,
			CrashMVBA:           newBC_m.CrashMVBA,
		}
	} else {
		fmt.Println(newBC_m.BroadcastType)
		panic("wrong bctype")
	}

	newBC.net.Init()

	if !newBC.Testmode || newBC_m.BroadcastType == "WRBC" {
		//open database
		dbpath := fmt.Sprintf("%s%sbroadcast/%d-%d", gopath, newBC_m.DBPath, lid, nid)
		//dbpath := gopath + "/src/super_fabric/sample_sf/database/leveldb/db"
		db := leveldb.CreateDB(dbpath)
		err = os.Mkdir(dbpath, 0750)
		if err != nil && !os.IsExist(err) {
			panic(err)
		}
		db.Open()
		newBC.DB = db
		defer db.Close()
	} else {
		newBC.DB = &leveldb.DB{}
	}

	if newBC_m.BroadcastType == "RBC" {
		for i := 0; i < newBC.node_num; i++ {
			newBC.conMsgBuf_rbc[i] = make(chan rbc.SendMsg, 500)
		}
	} else if newBC_m.BroadcastType == "CBC" {
		for i := 0; i < newBC.node_num; i++ {
			newBC.conMsgBuf_cbc[i] = make(chan cbc.SendMsg, 500)
		}
	} else if newBC_m.BroadcastType == "WRBC" {
		for i := 0; i < newBC.node_num; i++ {
			newBC.conMsgBuf_wrbc[i] = make(chan wrbc.SendMsg, 500)
		}
	} else {
		panic("wrong bctype")
	}

	//initial network

	//dial order_m
	go func() {
		time.Sleep(time.Duration(sleeptime) * time.Second)
		con, err := network.Dial(ordermIP)
		fmt.Println("dial:", ordermIP)
		if err != nil {
			panic(err)
		} else {
			newBC.orderCon = con
		}
	}()

	//start BC
	go newBC.handle_msg2orderCH()
	if lid == nid {
		ipmPath := fmt.Sprintf("%s%sipm.txt", gopath, newBC_m.BCIPPath)
		fi, err := os.Open(ipmPath)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}

		br := bufio.NewReader(fi)
		var bcmIP string

		for i := 0; i < 1; i++ {
			a, _, c := br.ReadLine()
			if c == io.EOF {
				break
			}
			if i == newBC.sid-1 {
				bcmIP = string(a)
				break
			}
		}
		fi.Close()
		newBC.start_leader(bcmIP)
	} else {
		newBC.start_follower()
	}

}

func (bc *BC) handle_signal2tp() {
	for {
		signal := <-bc.signal2tpCH
		if bc.IsControlBatch {
			err := bc.net.Send(bc.tpCon, signal)
			if err != nil {
				panic(err)
			}
		}
		fmt.Println("send a signal")
	}
}
func (bc *BC) handle_msg2orderCH() {
	//send result to order
	//dropflag := false
	for {
		msg := <-bc.msg2order

		//if !dropflag {
		waitcount := 0
		for {
			err := bc.net.Send(bc.orderCon, msg)
			if err != nil {
				for {
					con, err := network.Dial(bc.ordermIP)
					fmt.Println("dial:", bc.ordermIP)
					if err != nil {
						time.Sleep(time.Duration(1) * time.Second)
						waitcount++
						if waitcount > 5 {
							panic(fmt.Sprintln("fail to reconnect to ", bc.ordermIP))
							//dropflag = true
							//break
						}
					} else {
						bc.orderCon = con
						break
					}
				}

			} else {
				break
			}

		}
		//}

		if bc.height%10 == 0 {
			fmt.Println(time.Now(), "send a blk to order in height:", bc.height)
		}
		//fmt.Println("send a blk to order in height:%d\n", bc.height)
		bc.height++
	}

}

func (bc *BC) start_leader(bcmIP string) {
	//init network
	//listen msg from txpool
	//bcmIP := fmt.Sprintf("%s%d%s", bc.ips[bc.nid-1][:12], 1, bc.ips[bc.nid-1][13:])

	go network.Listen_msg(bcmIP, bc.txmsgIn, true)

	//listen msg from followers
	go network.Listen_msg(bc.ips[bc.nid-1], bc.bcmsgIn, true)

	//dial followers
	/*go func() {
		time.Sleep(time.Duration(20) * time.Second)
		for i := 0; i < bc.node_num; i++ {
			if i != bc.nid-1 {
				go func(i int) {
					con, err := network.Dial(bc.ips[i])
					fmt.Println("dial:", bc.ips[i])
					if err != nil {
						panic(err)
					} else {
						bc.bc2fCon = append(bc.bc2fCon, con)
					}
				}(i)
			}

		}
	}()*/

	go bc.handle_msgOutCH()
	for i := 1; i <= bc.node_num; i++ {
		if i != bc.nid {
			go bc.handle_con(i)
		}
	}

	//dial txpool
	go func() {
		time.Sleep(time.Duration(sleeptime) * time.Second)
		con, err := network.Dial(bc.txpoolIP)
		fmt.Println("dial:", bc.txpoolIP)
		if err != nil {
			panic(err)
		} else {
			bc.tpCon = con
		}

	}()

	go bc.handle_signal2tp()

	if bc.bctype == "RBC" {
		leaderlogfile, err := os.OpenFile(fmt.Sprintf("./log/leader%d-%d.txt", bc.nid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			panic(err)
		}
		leaderlog := log.New(leaderlogfile, "leader:", log.LstdFlags)
		newBCl := rbc.NewBroadcast_leader(bc.nid, bc.lid, bc.sid, bc.node_num, bc.txmsgIn, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_rbc, *leaderlog, *bc.DB, bc.Testmode, bc.BatchSize, bc.signal2tpCH)
		newBCl.Start()
	} else if bc.bctype == "CBC" {
		newBCl := cbc.NewBroadcast_leader(bc.nid, bc.lid, bc.sid, bc.node_num, bc.sigmeta, bc.txmsgIn, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_cbc, bc.DB, bc.Testmode, bc.BatchSize, bc.signal2tpCH)
		newBCl.Start()
	} else if bc.bctype == "WRBC" {
		leaderlogfile, err := os.OpenFile(fmt.Sprintf("./log/leader%d-%d.txt", bc.nid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			panic(err)
		}
		leaderlog := log.New(leaderlogfile, "leader:", log.LstdFlags)
		newBCl := wrbc.NewBroadcast_leader(bc.nid, bc.lid, bc.sid, bc.node_num, bc.txmsgIn, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_wrbc, *leaderlog, *bc.DB, bc.Testmode, bc.BatchSize, bc.signal2tpCH)
		newBCl.Start()
	} else {
		panic("wrong bctype")
	}
	/*leaderlogfile, err := os.OpenFile(fmt.Sprintf("./log/leader%d-%d.txt", bc.nid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	leaderlog := log.New(leaderlogfile, "leader:", log.LstdFlags)
	newBCl := broadcast.NewBroadcast_leader(bc.nid, bc.lid, bc.sid, bc.node_num, bc.txmsgIn, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut, *leaderlog)*/
	/*newBCl := broadcast.NewBroadcast_leader(bc.nid, bc.lid, bc.sid, bc.node_num, bc.sigmeta, bc.txmsgIn, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut, bc.DB, bc.Testmode)
	newBCl.Start()*/

}

func (bc *BC) start_follower() {

	//initial network
	//listen msg from leader
	go network.Listen_msg(bc.ips[bc.nid-1], bc.bcmsgIn, true)
	/*go func() {
		time.Sleep(time.Duration(20) * time.Second)
		for i := 1; i <= bc.node_num; i++ {
			con, err := network.Dial(bc.ips[i-1])
			fmt.Println("dial:", bc.ips[i-1])
			if err != nil {
				panic(err)
			} else {
				bc.bc2fCon = append(bc.bc2fCon, con)
			}
		}

	}()*/
	go bc.handle_msgOutCH()
	for i := 1; i <= bc.node_num; i++ {
		if i != bc.nid {
			go bc.handle_con(i)
		}
	}

	if bc.bctype == "RBC" {
		followerlogfile, err := os.OpenFile(fmt.Sprintf("./log/follower%d-%d-%d.txt", bc.nid, bc.lid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			panic(err)
		}
		followerlog := log.New(followerlogfile, "follower:", log.LstdFlags)
		newBCf := rbc.NewBroadcast_follower(bc.nid, bc.lid, bc.sid, bc.node_num, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_rbc, *followerlog, *bc.DB, bc.Testmode)
		newBCf.Start()
	} else if bc.bctype == "CBC" {
		newBCf := cbc.NewBroadcast_follower(bc.nid, bc.lid, bc.sid, bc.node_num, bc.sigmeta, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_cbc, bc.DB, bc.Testmode)
		newBCf.Start()
	} else if bc.bctype == "WRBC" {
		followerlogfile, err := os.OpenFile(fmt.Sprintf("./log/follower%d-%d-%d.txt", bc.nid, bc.lid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
		if err != nil {
			panic(err)
		}
		followerlog := log.New(followerlogfile, "follower:", log.LstdFlags)
		newBCf := wrbc.NewBroadcast_follower(bc.nid, bc.lid, bc.sid, bc.node_num, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut_wrbc, *followerlog, *bc.DB, bc.Testmode)
		newBCf.Start()
	} else {
		panic("wrong bctype")
	}
	/*followerlogfile, err := os.OpenFile(fmt.Sprintf("./log/follower%d-%d-%d.txt", bc.nid, bc.lid, bc.k), os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
	if err != nil {
		panic(err)
	}
	followerlog := log.New(followerlogfile, "follower:", log.LstdFlags)
	newBCf := broadcast.NewBroadcast_follower(bc.nid, bc.lid, bc.sid, bc.node_num, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut, *followerlog)*/
	/*newBCf := broadcast.NewBroadcast_follower(bc.nid, bc.lid, bc.sid, bc.node_num, bc.sigmeta, bc.msg2order, bc.bcmsgIn, bc.bcmsgOut, bc.DB, bc.Testmode)
	newBCf.Start()*/
}

func (bc *BC) handle_msgOutCH() {
	//routing msgs
	if bc.bctype == "RBC" {
		for {
			msg := <-bc.bcmsgOut_rbc
			if msg.ID > bc.node_num || msg.ID <= 0 {
				panic("wrong msg ID")
			}
			//fmt.Println("newstart: get a msg from protocol to ", msg.ID, "of type ", msg.Type)
			bc.conMsgBuf_rbc[msg.ID-1] <- msg
		}
	} else if bc.bctype == "CBC" {
		for {
			msg := <-bc.bcmsgOut_cbc
			if msg.ID > bc.node_num || msg.ID <= 0 {
				panic("wrong msg ID")
			}
			//fmt.Println("newstart: get a msg from protocol to ", msg.ID, "of type ", msg.Type)
			bc.conMsgBuf_cbc[msg.ID-1] <- msg
		}
	} else if bc.bctype == "WRBC" {
		for {
			msg := <-bc.bcmsgOut_wrbc
			if msg.ID > bc.node_num || msg.ID <= 0 {
				panic("wrong msg ID")
			}
			//fmt.Println("newstart: get a msg from protocol to ", msg.ID, "of type ", msg.Type)
			bc.conMsgBuf_wrbc[msg.ID-1] <- msg
		}
	} else {
		panic("wrong bctype")
	}
	/*for {
		msg := <-bc.bcmsgOut
		if msg.ID > bc.node_num || msg.ID <= 0 {
			panic("wrong msg ID")
		}
		//fmt.Println("newstart: get a msg from protocol to ", msg.ID, "of type ", msg.Type)
		bc.conMsgBuf[msg.ID-1] <- msg
	}*/

}

func (bc *BC) handle_con(id int) {
	fmt.Println("inside handle_con ", id)
	time.Sleep(time.Duration(sleeptime) * time.Second)
	for {
		con, err := network.Dial(bc.ips[id-1])
		fmt.Println("dial:", bc.ips[id-1])
		if err != nil {
			fmt.Println(err)
			time.Sleep(time.Duration(1) * time.Second)
		} else {
			fmt.Println("dial:", bc.ips[id-1], "done")
			bc.bc2fCon[id-1] = con
			bc.handle_conMsg(id)
			con.Close()
		}
		time.Sleep(time.Duration(1) * time.Second)
	}

}

func (bc *BC) handle_conMsg(id int) {
	fmt.Println("inside handle_conMsg ", id)
	if bc.bctype == "RBC" {
		//protoMsg := make(chan rbc.SendMsg, 100)
		//recovMsg := make(chan int, 500)
		//callhelpMsg := make(chan rbc.SendMsg, 500)
		dropflag := false
		waitcount := 0
		for {
			var msg rbc.SendMsg
			//var msgtype string
			/*select {
			case msg = <-protoMsg:
				//msgtype = "protoMsg"
			case height := <-recovMsg:
				//msgtype = "recovMsg"
				bcblock, err := bc.DB.Get(IntToBytes(height))
				if err != nil {
					panic(err)
				}
				//send bcblock back
				bcmsg := &pb.BCMsg{
					Type:    int32(4),
					Content: bcblock,
				}
				bcmsgbyte, err := proto.Marshal(bcmsg)
				if err != nil {
					panic(err)
				}
				msg = rbc.SendMsg{
					ID:      id,
					Type:    1,
					Content: bcmsgbyte,
					Height:  height,
				}
			case msg = <-callhelpMsg:
				//msgtype = "callhelpMsg"
			default:
				//msgtype = "conMsgBuf"
				msg = <-bc.conMsgBuf_rbc[id-1]
			}*/
			msg = <-bc.conMsgBuf_rbc[id-1]
			if !dropflag {
				for {
					err := bc.net.Send(bc.bc2fCon[id-1], msg.Content)
					if err != nil {
						for {
							con, err := network.Dial(bc.ips[id-1])
							fmt.Println("dial:", bc.ips[id-1])
							if err != nil {
								time.Sleep(time.Duration(1) * time.Second)
								waitcount++
								if waitcount > 5 {
									fmt.Println("fail to reconnect to ", bc.bc2fCon[id-1])
									dropflag = true
									break
								}
							} else {
								bc.bc2fCon[id-1] = con
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
		}
	} else if bc.bctype == "CBC" {
		protoMsg := make(chan cbc.SendMsg, 100)
		recovMsg := make(chan int, 500)
		callhelpMsg := make(chan cbc.SendMsg, 500)
		dropflag := false
		waitcount := 0
		if id > bc.node_num-bc.CrashMVBA {
			for {
				<-bc.conMsgBuf_cbc[id-1]
			}
		}

		if bc.nid > bc.node_num-bc.CrashMVBA {
			for {
				<-bc.conMsgBuf_cbc[id-1]
			}
		}
		for {
			var msg cbc.SendMsg
			//var msgtype string
			select {
			case msg = <-protoMsg:
				//msgtype = "protoMsg"
			case height := <-recovMsg:
				//msgtype = "recovMsg"
				bcblock, err := bc.DB.Get(IntToBytes(height))
				if err != nil {
					panic(err)
				}
				//send bcblock back
				bcmsg := &pb.BCMsg{
					Type:    int32(4),
					Content: bcblock,
				}
				bcmsgbyte, err := proto.Marshal(bcmsg)
				if err != nil {
					panic(err)
				}
				msg = cbc.SendMsg{
					ID:      id,
					Type:    1,
					Content: bcmsgbyte,
					Height:  height,
				}
			case msg = <-callhelpMsg:
				//msgtype = "callhelpMsg"
			default:
				//msgtype = "conMsgBuf"
				msg = <-bc.conMsgBuf_cbc[id-1]
			}
			if !dropflag {
				for {
					err := bc.net.Send(bc.bc2fCon[id-1], msg.Content)
					if err != nil {
						for {
							con, err := network.Dial(bc.ips[id-1])
							fmt.Println("dial:", bc.ips[id-1])
							if err != nil {
								time.Sleep(time.Duration(1) * time.Second)
								waitcount++
								if waitcount > 5 {
									dropflag = true
									break
								}
							} else {
								bc.bc2fCon[id-1] = con
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

			/*err := network.Send(bc.bc2fCon[id-1], msg.Content)
			if err != nil {
				fmt.Println("start reconnect")
				close := make(chan bool)
				//reconnect
				go bc.handle_reconnect(id, close)
				//buffer msg
				bc.handle_buffConMsg(id, close, msg, protoMsg, recovMsg, callhelpMsg)
			}*/
			//fmt.Println("done sending a msg to ", id, " of type ", msgtype)
		}
	} else if bc.bctype == "WRBC" {
		//protoMsg := make(chan wrbc.SendMsg, 100)
		//recovMsg := make(chan int, 500)
		//callhelpMsg := make(chan wrbc.SendMsg, 500)
		dropflag := false
		waitcount := 0
		if id > bc.node_num-bc.CrashMVBA {
			for {
				<-bc.conMsgBuf_wrbc[id-1]
			}
		}

		if bc.nid > bc.node_num-bc.CrashMVBA {
			for {
				<-bc.conMsgBuf_wrbc[id-1]
			}
		}
		for {
			var msg wrbc.SendMsg
			msg = <-bc.conMsgBuf_wrbc[id-1]
			if id != bc.nid {
				if !dropflag {
					for {
						err := bc.net.Send(bc.bc2fCon[id-1], msg.Content)
						if err != nil {
							for {
								con, err := network.Dial(bc.ips[id-1])
								fmt.Println("dial:", bc.ips[id-1])
								if err != nil {
									time.Sleep(time.Duration(1) * time.Second)
									waitcount++
									if waitcount > 5 {
										dropflag = true
										break
									}
								} else {
									bc.bc2fCon[id-1] = con
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
			}

		}
	} else {
		panic("wrong bctype")
	}
	/*protoMsg := make(chan broadcast.SendMsg, 100)
	recovMsg := make(chan int, 500)
	callhelpMsg := make(chan broadcast.SendMsg, 500)
	for {
		var msg broadcast.SendMsg
		//var msgtype string
		select {
		case msg = <-protoMsg:
			//msgtype = "protoMsg"
		case height := <-recovMsg:
			//msgtype = "recovMsg"
			bcblock, err := bc.DB.Get(IntToBytes(height))
			if err != nil {
				panic(err)
			}
			//send bcblock back
			bcmsg := &pb.BCMsg{
				Type:    int32(4),
				Content: bcblock,
			}
			bcmsgbyte, err := proto.Marshal(bcmsg)
			if err != nil {
				panic(err)
			}
			msg = broadcast.SendMsg{
				ID:      id,
				Type:    1,
				Content: bcmsgbyte,
				Height:  height,
			}
		case msg = <-callhelpMsg:
			//msgtype = "callhelpMsg"
		default:
			//msgtype = "conMsgBuf"
			msg = <-bc.conMsgBuf[id-1]
		}
		bc.net.Send(bc.bc2fCon[id-1], msg.Content)
		//err := network.Send(bc.bc2fCon[id-1], msg.Content)
		//if err != nil {
		//	fmt.Println("start reconnect")
		//	close := make(chan bool)
		//	//reconnect
		//	go bc.handle_reconnect(id, close)
		//	//buffer msg
		//	bc.handle_buffConMsg(id, close, msg, protoMsg, recovMsg, callhelpMsg)
		//}
		//fmt.Println("done sending a msg to ", id, " of type ", msgtype)
	}*/

}

func (bc *BC) handle_reconnect(id int, close chan bool) {
	for {
		con, err := network.Dial(bc.ips[id-1])
		fmt.Println("dial:", bc.ips[id-1])
		if err != nil {
			panic(err)
		} else {
			bc.bc2fCon[id-1] = con
			SafeClose(close)
		}

		time.Sleep(time.Duration(1) * time.Second)
	}

}

/*func (bc *BC) handle_buffConMsg(id int, close chan bool, initMsg broadcast.SendMsg, protoMsg chan broadcast.SendMsg, recovMsg chan int, callhelpMsg chan broadcast.SendMsg) {
	fmt.Println("inside handle_buffConMsg")
	switch initMsg.Type {
	case 0:
		protoMsg <- initMsg
	case 1:
		recovMsg <- initMsg.Height
	case 2:
		callhelpMsg <- initMsg
	default:
		panic("wrong type msg")
	}

	for {
		select {
		case <-close:
			return
		default:
		}
		msg := <-bc.conMsgBuf[id-1]
		switch msg.Type {
		case 0:
			<-protoMsg
			protoMsg <- msg
		case 1:
			recovMsg <- msg.Height
		case 2:
			callhelpMsg <- msg
		default:
			panic("wrong type msg")
		}
	}

}*/

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

