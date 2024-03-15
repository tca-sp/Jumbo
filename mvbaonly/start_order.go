package main

import (
	"bufio"
	"bytes"
	cy "dumbo_fabric/crypto/signature"
	bls "dumbo_fabric/crypto/signature/bls"
	ec "dumbo_fabric/crypto/signature/ecdsa"
	schnorr "dumbo_fabric/crypto/signature/schnorr"
	aggregate "dumbo_fabric/crypto/signature/schnorr_aggregate"
	mvba "dumbo_fabric/mvbaonly/smvba"
	"dumbo_fabric/network"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/golang/protobuf/proto"

	mapset "github.com/deckarep/golang-set"
)

const mspid = "SampleOrg"

var sleeptime int

var local bool

type order_m struct {
	Order_m       string `yaml:"Order_m"`
	Node_num      int    `yaml:"Node_num"`
	K             int    `yaml:"K"`
	SignatureType string `yaml:"SignatureType"`
	Round         int
	ID            int
	rcvOrderCH    chan []byte //buff msg rcv from all order
	rcvOutputCH   chan bool
	sendIntputCH  chan []byte     //share channel: send input to my order
	smvbaCH       chan []byte     //share channel: proto msgs from other orders
	protoMsgOut   chan pb.SendMsg //share channel: proto msgs to other orders

	dumbomvbaCH chan []byte //share channel: dumbomvba msgs from other orders
	baCH        chan []byte //share channel: ba msgs from other orders
	mRBCCH      chan []byte //share channel: rbcwithfinish msgs from other orders
	msgOutCH    chan pb.SendMsg
	conMsgBuff  []chan pb.SendMsg

	ready bool

	OrderIPPath    string `yaml:"OrderIPPath"`
	SkPath         string `yaml:"SkPath"`
	PkPath         string `yaml:"PkPath"`
	sigmeta        cy.Signature
	ips            []string
	orderCons      []net.Conn
	net            network.Network
	UsingDumboMVBA bool   `yaml:"UsingDumboMVBA"`
	BroadcastType  string `yaml:"BroadcastType"`
	MVBAType       string `yaml:"MVBAType"`
	Sleeptime      int    `yaml:"Sleeptime"`
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

	newOrder_m.ID = id
	newOrder_m.rcvOrderCH = make(chan []byte, 3000)
	newOrder_m.rcvOutputCH = make(chan bool, 2)
	newOrder_m.sendIntputCH = make(chan []byte, 3000)
	newOrder_m.smvbaCH = make(chan []byte, 3000)
	newOrder_m.protoMsgOut = make(chan pb.SendMsg, 10000)
	newOrder_m.dumbomvbaCH = make(chan []byte, 3000)
	newOrder_m.baCH = make(chan []byte, 3000)
	newOrder_m.mRBCCH = make(chan []byte, 3000)
	newOrder_m.msgOutCH = make(chan pb.SendMsg, 3000)
	newOrder_m.conMsgBuff = make([]chan pb.SendMsg, newOrder_m.Node_num)
	for i := 0; i < newOrder_m.Node_num; i++ {
		newOrder_m.conMsgBuff[i] = make(chan pb.SendMsg, 3000)
	}
	//newOrder_m.cutBlockCH = make(chan cut, 100)
	//newOrder_m.oldBCBlocks.timestamps = make([][][]int64, newOrder_m.Node_num)
	newOrder_m.orderCons = make([]net.Conn, newOrder_m.Node_num)
	newOrder_m.net = network.New(false, 1, false, 1)
	newOrder_m.net.Init()

	newOrder_m.ready = true

	//read order ips
	ipPath := fmt.Sprintf("%s%sip.txt", gopath, newOrder_m.OrderIPPath)
	fi, err := os.Open(ipPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)
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

	//receive msg from other orderes
	go network.Listen_msg(orderip, newOrder_m.rcvOrderCH, true)
	//deal with msg from other orders
	go newOrder_m.handle_rcvOrderCH()
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

	newOrder := mvba.NewOrder(id, newOrder_m.Node_num, newOrder_m.K, orderip, ips, newOrder_m.sigmeta, newOrder_m.sendIntputCH, newOrder_m.rcvOutputCH, check_inputs, check_input_RBC, check_inputs_QCagg, newOrder_m.smvbaCH, newOrder_m.protoMsgOut, newOrder_m.UsingDumboMVBA, newOrder_m.dumbomvbaCH, newOrder_m.BroadcastType, newOrder_m.MVBAType, newOrder_m.baCH, newOrder_m.mRBCCH)
	newOrder.Start()

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
		case 4: //get a dumbomvba msg
			od_m.dumbomvbaCH <- ordermsg.Content
		case 5: //bamsg for fin
			od_m.baCH <- ordermsg.Content
		case 6: //modified rbc msg for fin
			od_m.mRBCCH <- ordermsg.Content
		default:
			panic("wrong type order msg")
		}

	}
}

//receive an output from MVBA
func (od_m *order_m) handle_rcvOutputCH() {
	time.Sleep(time.Duration(sleeptime+10) * time.Second)
	if od_m.MVBAType=="normal"{
		od_m.sendIntputCH <- make([]byte, 13000)
	}else{
		od_m.sendIntputCH <- make([]byte, 196*4)
	}
	starttime := time.Now()
	count := 0
	for {
		<-od_m.rcvOutputCH
		timetmp := time.Now()
		fmt.Println(timetmp, "finish a round of mvba")

		elapsed := timetmp.Sub(starttime)
		count++
		fmt.Println("all latency:", elapsed/time.Duration(count))
	if od_m.MVBAType=="normal"{
		od_m.sendIntputCH <- make([]byte, 13000)
	}else{
		od_m.sendIntputCH <- make([]byte, 196*4)
	}
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
			<-od_m.conMsgBuff[msg.ID-1]
			od_m.conMsgBuff[msg.ID-1] <- msg
		}

	}
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
	}
}

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
