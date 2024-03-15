package main

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	//broadcast "dumbo_fabric/broadcaster/broadcast/rbc"
	//broadcast "dumbo_fabric/broadcaster/broadcast/cbc"
	cbc "dumbo_fabric/SimpleHotStuff/cbc"
	cy "dumbo_fabric/crypto/signature"
	bls "dumbo_fabric/crypto/signature/bls"
	ec "dumbo_fabric/crypto/signature/ecdsa"
	schnorr "dumbo_fabric/crypto/signature/schnorr"
	aggregate "dumbo_fabric/crypto/signature/schnorr_aggregate"

	"dumbo_fabric/network"
	pb "dumbo_fabric/struct"

	"gopkg.in/yaml.v2"
)

var sleeptime int

type BC_m struct {
	Node_num      int    `yaml:"Node_num"`
	BatchSize     int    `yaml:"BatchSize"`
	PkPath        string `yaml:"PkPath"`
	SkPath        string `yaml:"SkPath"`
	BCIPPath      string `yaml:"BCIPPath"`
	SignatureType string `yaml:"SignatureType"`
	Sleeptime     int    `yaml:"Sleeptime"`
}

type BC struct {
	id        int
	node_num  int
	k         int
	BatchSize int
	sigmeta   cy.Signature
	ips       []string
	txmsgIn   chan []byte     //txmsg
	outputCH  chan pb.BCBlock //receive valid block
	bcmsgIn   chan []byte     //msg from other broadcast

	bcmsgOut_cbc chan cbc.SendMsg
	bc2fCon      []net.Conn

	conMsgBuf_cbc []chan cbc.SendMsg
	height        int
	net           network.Network
}

func main() {
	var nidf = flag.Int("nid", 0, "-nid")
	flag.Parse()

	id := *nidf

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
	signature.Init(gopath+newBC_m.PkPath, newBC_m.Node_num, id)

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
		ips[i] = string(a) + fmt.Sprintf(":%d", 14000)
	}
	fi.Close()

	newBC := &BC{
		id:            id,
		node_num:      newBC_m.Node_num,
		BatchSize:     newBC_m.BatchSize,
		sigmeta:       signature,
		ips:           ips,
		txmsgIn:       make(chan []byte, 2000),
		outputCH:      make(chan pb.BCBlock, 100),
		bcmsgIn:       make(chan []byte, 500),
		bcmsgOut_cbc:  make(chan cbc.SendMsg, 500),
		bc2fCon:       make([]net.Conn, newBC_m.Node_num),
		height:        0,
		conMsgBuf_cbc: make([]chan cbc.SendMsg, newBC_m.Node_num),
	}

	newBC.net.Init()

	for i := 0; i < newBC.node_num; i++ {
		newBC.conMsgBuf_cbc[i] = make(chan cbc.SendMsg, 500)
	}

	//initial network

	if id == 1 {
		newBC.start_leader()
	} else {
		newBC.start_follower()
	}

}

func (bc *BC) start_leader() {
	go network.Listen_msg(bc.ips[bc.id-1], bc.bcmsgIn, true)

	go bc.handle_msgOutCH()
	for i := 1; i <= bc.node_num; i++ {
		if i != bc.id {
			go bc.handle_con(i)
		}
	}

	go bc.generateTxBlock()

	newBCl := cbc.NewBroadcast_leader(bc.id, bc.node_num, bc.sigmeta, bc.txmsgIn, bc.outputCH, bc.bcmsgIn, bc.bcmsgOut_cbc, bc.BatchSize)
	newBCl.Start()

}

func (bc *BC) start_follower() {

	go network.Listen_msg(bc.ips[bc.id-1], bc.bcmsgIn, true)

	go bc.handle_msgOutCH()
	for i := 1; i <= bc.node_num; i++ {
		if i != bc.id {
			go bc.handle_con(i)
		}
	}

	newBCf := cbc.NewBroadcast_follower(bc.id, bc.node_num, bc.sigmeta, bc.outputCH, bc.bcmsgIn, bc.bcmsgOut_cbc)
	newBCf.Start()

}

func (bc *BC) handle_outputCH() {

	var newtimestamp time.Time
	var locktimestamp time.Time
	count := 0
	var alllatency time.Duration

	firsttime := true
	txcount := 0
	var starttime time.Time

	for {
		outblock := <-bc.outputCH
		if firsttime {
			starttime = time.Now()
			firsttime = false
		}
		blocktimetmp := time.Unix(0, outblock.RawBC.Timestamp)

		if bc.height > 0 {
			txcount += bc.BatchSize
		}

		if bc.height > 1 {
			elapsed := time.Now().Sub(starttime)
			latency := time.Now().Sub(locktimestamp)
			alllatency += latency
			count++
			fmt.Println("all latency:", alllatency/time.Duration(count))
			fmt.Println("all tps:", float64(txcount)/(float64(elapsed)/float64(time.Second)))
		}

		locktimestamp = newtimestamp
		newtimestamp = blocktimetmp

	}

}

func (bc *BC) generateTxBlock() {

	for {

		block := make([]byte, bc.BatchSize*250)

		for i := 0; i < bc.BatchSize; i++ {
			block[250*i] = 100
		}

		bc.txmsgIn <- block

	}
}

func (bc *BC) handle_msgOutCH() {
	//routing msgs

	for {
		msg := <-bc.bcmsgOut_cbc
		if msg.ID > bc.node_num || msg.ID <= 0 {
			panic("wrong msg ID")
		}
		//fmt.Println("newstart: get a msg from protocol to ", msg.ID, "of type ", msg.Type)
		bc.conMsgBuf_cbc[msg.ID-1] <- msg
	}

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
	dropflag := false
	waitcount := 0
	for {
		msg := <-bc.conMsgBuf_cbc[id-1]
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
