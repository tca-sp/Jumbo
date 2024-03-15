package main

import (
	"bufio"
	rbc "dumbo_fabric/fin/rbc"
	sfm "dumbo_fabric/fin/signaturefreemvba"
	"dumbo_fabric/network"
	pb "dumbo_fabric/struct"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"gopkg.in/yaml.v2"
)

var sleeptime int

var local bool

type order struct {
	Node_num     int `yaml:"Node_num"`
	Round        int
	ID           int
	ClientIPPath string `yaml:"ClientIPPath"`
	BCIPPath     string `yaml:"BCIPPath"`
	OrderIPPath  string `yaml:"OrderIPPath"`
	TpIPPath     string `yaml:"TpIPPath"`
	SkPath       string `yaml:"SkPath"`
	PkPath       string `yaml:"PkPath"`
	Sleeptime    int    `yaml:"Sleeptime"`
	BatchSize    int    `yaml:"BatchSize"`

	net network.Network

	ordermip  string
	orderip   string
	tpIP      string
	ips       []string
	orderCons []net.Conn
	tpCon     net.Conn

	rcvOrderCH  chan []byte
	rcvClientCH chan []byte
	ctrbcCH     chan []byte
	rbcCH       chan []byte
	baCH        chan []byte
	signal2tpCH chan []byte

	msgoutCH   chan pb.SendMsg
	conMsgBuff []chan pb.SendMsg
}

func main() {
	local = false
	var idf = flag.Int("id", 0, "-id")
	flag.Parse()
	id := *idf

	//read node.yaml
	newOrder := &order{}
	gopath := os.Getenv("GOPATH")
	readBytes, err := ioutil.ReadFile(gopath + "/src/dumbo_fabric/config/node.yaml")
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(readBytes, newOrder)
	if err != nil {
		panic(err)
	}
	newOrder.ID = id
	newOrder.Round = 0
	newOrder.net.Init()
	newOrder.orderCons = make([]net.Conn, newOrder.Node_num)
	newOrder.rcvOrderCH = make(chan []byte, 10000)
	newOrder.rcvClientCH = make(chan []byte, 10)
	newOrder.ctrbcCH = make(chan []byte, 300000)
	newOrder.rbcCH = make(chan []byte, 300000)
	newOrder.baCH = make(chan []byte, 100000)
	newOrder.signal2tpCH = make(chan []byte, 10)
	newOrder.msgoutCH = make(chan pb.SendMsg, 100000)
	newOrder.conMsgBuff = make([]chan pb.SendMsg, newOrder.Node_num)
	for i := 0; i < newOrder.Node_num; i++ {
		newOrder.conMsgBuff[i] = make(chan pb.SendMsg, 100000)
	}

	sleeptime = newOrder.Sleeptime

	//read ip_m: receive msg from txpool
	ipmPath := fmt.Sprintf("%s%sipm.txt", gopath, newOrder.BCIPPath)
	fi, err := os.Open(ipmPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)

	a, _, c := br.ReadLine()
	if c == io.EOF {
		panic("EOF")
	}
	newOrder.ordermip = string(a)

	fi.Close()

	TpIPPath := fmt.Sprintf("%s%sipm.txt", gopath, newOrder.TpIPPath)
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
	newOrder.tpIP = tpIP

	//read order ips
	ipPath := fmt.Sprintf("%s%sip.txt", gopath, newOrder.OrderIPPath)
	fi, err = os.Open(ipPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br = bufio.NewReader(fi)
	ips := make([]string, newOrder.Node_num)
	for i := 0; i < newOrder.Node_num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}

		ips[i] = string(a) + fmt.Sprintf(":%d", 12000)
		if i == id-1 {
			newOrder.orderip = string(a) + fmt.Sprintf(":%d", 12000)
		}

	}
	fi.Close()
	newOrder.ips = ips

	newOrder.InitNetwork(newOrder.ID)

	go newOrder.routeordermsg()
	go newOrder.handle_msgOutCH()
	go newOrder.handle_signal2tp()

	ctrbcfuturebuf := make(chan pb.RBCMsg, 1000)
	bafuturebuf := make(chan pb.BAMsg, 1000)
	rbcfuturebuf := make(chan pb.RBCMsg, 1000)
	baoldbuf := make(chan pb.BAMsg, 1000)
	ctrbcoldbuf := make(chan pb.RBCMsg, 1000)
	rbcoldbuf := make(chan pb.RBCMsg, 1000)

	var alllatency time.Duration
	var allblockcount int
	var alltxcount int
	var launchtime time.Time
	isfirsttime := true
	islastcontainmine := true //if true, generate a new input
	var input []byte
	for {

		if islastcontainmine {
			fmt.Println("fetch a new input")
			//txblk := <-newOrder.rcvClientCH
			var txblk []byte
			if isfirsttime {
				txblk = <-newOrder.rcvClientCH
				//launchtime = time.Now()
				//isfirsttime = false
			} else {
				select {
				case txblk = <-newOrder.rcvClientCH:
				default:
					newOrder.signal2tpCH <- make([]byte, 1)
					txblk = <-newOrder.rcvClientCH
				}
			}

			timestamp := time.Now().UnixNano()
			bcblk := pb.BCBlock{
				RawBC:   &pb.RawBC{Timestamp: timestamp},
				Payload: txblk,
			}
			input, err = proto.Marshal(&bcblk)
			if err != nil {
				panic(err)
			}
			fmt.Println("fetch a new input done")

		} else {
			fmt.Println("use old input")
		}


		fmt.Println("start a new round at:", time.Now())

		done := make(chan bool)
		//start ctrbc
		ctrbcmsgCH := make([]chan pb.RBCMsg, newOrder.Node_num)
		for i := 0; i < newOrder.Node_num; i++ {
			ctrbcmsgCH[i] = make(chan pb.RBCMsg, 1000)
		}
		checkrbcbuf(ctrbcoldbuf, ctrbcfuturebuf, newOrder.Round)
		ctrbcoldbuf = ctrbcfuturebuf
		ctrbcfuturebuf = make(chan pb.RBCMsg, 90000)
		ctoutputsCH := make(chan pb.RBCOut, newOrder.Node_num)
		go newOrder.handle_ctrbcmsgCH(newOrder.Round, done, ctrbcmsgCH, ctrbcoldbuf, ctrbcfuturebuf)

		fakedone := make(chan bool)
		go newOrder.start_ctrbcs(input, ctoutputsCH, ctrbcmsgCH, fakedone)
		ctrbcoutputs := make([]bool, newOrder.Node_num)
		ctrbcoutputscontent := make([]pb.BCBlock, newOrder.Node_num)
		ctrbcdonecount := 0
		mvbaready := make(chan bool, 2)
		//wait for n-f ctrbc finished
		go func() {
			var output pb.RBCOut
			for {
				select {
				case <-done:
					return
				default:
					select {
					case <-done:
						return
					case output = <-ctoutputsCH:
						//fmt.Println("get an output from ready from", output.ID)
					}
					bcblk := pb.BCBlock{}
					err := proto.Unmarshal(output.Value, &bcblk)
					if err != nil {
						panic(err)
					}
					ctrbcdonecount++
					ctrbcoutputs[output.ID-1] = true
					ctrbcoutputscontent[output.ID-1] = bcblk
					if ctrbcdonecount == ((newOrder.Node_num-1)/3)*2+1 {
						mvbaready <- true
					}
					if ctrbcdonecount%10 == 0 {
						fmt.Println(ctrbcdonecount, "rbc done")
					}
				}
			}
		}()

		<-mvbaready

		mvbainput := boolArrayToByteArray(ctrbcoutputs)
		time.Sleep(time.Millisecond * 300)
		//start sigfreemvba

		fmt.Println("start a new signature free mvba of round", newOrder.Round)

		checkbabuf(baoldbuf, bafuturebuf, newOrder.Round)
		bamsgCH := make(chan pb.BAMsg, 10000)
		baoldbuf = bafuturebuf
		bafuturebuf = make(chan pb.BAMsg, 90000)
		go newOrder.handle_bamsgCH(newOrder.Round, done, bamsgCH, baoldbuf, bafuturebuf)

		checkrbcbuf(rbcoldbuf, rbcfuturebuf, newOrder.Round)
		rbcmsgCH := make(chan pb.RBCMsg, 90000)
		rbcoldbuf = rbcfuturebuf
		rbcfuturebuf = make(chan pb.RBCMsg, 90000)
		go newOrder.handle_rbcmsgCH(newOrder.Round, done, rbcmsgCH, rbcoldbuf, rbcfuturebuf)

		mvba2order := make(chan []byte, 2)
		mvba := sfm.New_mvba(newOrder.Node_num, 1, newOrder.ID, newOrder.Round, newOrder.msgoutCH, bamsgCH, rbcmsgCH, check_input_rbc, nil, mvbainput, mvba2order, ctrbcoutputs)

		go mvba.Launch()

		output := <-mvba2order
		fmt.Println("done a mvba of round ", newOrder.Round)

		newOrder.wait_rbc_done(output, ctrbcoutputs)

		//newCH := make(chan pb.Msg, 10000)
		//order.mvbaMsgCH = newCH

		//test
		/*testbools := make([]bool, newOrder.Node_num)
		for i := 0; i < newOrder.Node_num; i++ {
			testbools[i] = true
		}
		testboolbytes := boolArrayToByteArray(testbools)
		newOrder.wait_rbc_done(testboolbytes, ctrbcoutputs)*/

		//test done

		close(done)
		newOrder.Round++

		//caculate latency and throughput
		txcount := 0
		blockcount := 0
		var latencycount time.Duration
		endtime := time.Now()

		outputbools := byteArrayToBoolArray(output, newOrder.Node_num)
		for i := 0; i < newOrder.Node_num; i++ {
			if outputbools[i] {
				blockcount++
				txcount += len(ctrbcoutputscontent[i].Payload) / 250
				tmptime := time.Unix(0, ctrbcoutputscontent[i].RawBC.Timestamp)
				elapsed := endtime.Sub(tmptime)
				latencycount += elapsed
			}
		}
		fmt.Println("output:", outputbools)
		if outputbools[id-1] {
			islastcontainmine = true
		} else {
			islastcontainmine = false
		}

		//alllatency += latencycount
		//alltxcount += txcount
		//allblockcount += blockcount
		//fmt.Println("****************done a mvba at", endtime)
		//fmt.Println("all tps:", float64(alltxcount)/(float64(endtime.Sub(launchtime))/float64(time.Second)))
		//fmt.Println("all latency:", alllatency/time.Duration(allblockcount))
		if !isfirsttime {
			alllatency += latencycount
			alltxcount += txcount
			allblockcount += blockcount
			fmt.Println("all tps:", float64(alltxcount)/(float64(endtime.Sub(launchtime))/float64(time.Second)))
			fmt.Println("all latency:", alllatency/time.Duration(allblockcount))
		} else {

			launchtime = time.Now()
			isfirsttime = false
		}
	}

}

func (order *order) InitNetwork(id int) {
	go network.Listen_msg(order.ordermip, order.rcvClientCH, true)
	//receive msg from other orderes
	go network.Listen_msg(order.orderip, order.rcvOrderCH, true)

	time.Sleep(time.Duration(sleeptime) * time.Second)

	go func() {
		con, err := network.Dial(order.tpIP)
		fmt.Println("dial:", order.tpIP)
		if err != nil {
			panic(err)
		} else {
			order.tpCon = con
		}

	}()

	for i := 0; i < order.Node_num; i++ {
		if i+1 != order.ID {
			go order.handle_orderCon(i + 1)
		}

	}
}

func (od_m *order) handle_orderCon(id int) {
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

func (order *order) routeordermsg() {
	for {
		msg := <-order.rcvOrderCH
		ordermsg := pb.OrderMsg{}
		err := proto.Unmarshal(msg, &ordermsg)
		if err != nil {
			panic(err)
		}

		switch ordermsg.Type {
		case 1:
			select {
			case order.ctrbcCH <- ordermsg.Content:
			default:
				fmt.Println("ctrbc buffer full")
				order.ctrbcCH <- ordermsg.Content
			}

		case 5:
			select {
			case order.baCH <- ordermsg.Content:
			default:
				fmt.Println("ba buffer full")
				order.baCH <- ordermsg.Content
			}

		case 6:
			select {
			case order.rbcCH <- ordermsg.Content:
			default:
				fmt.Println("rbc buffer full")
				order.rbcCH <- ordermsg.Content
			}

		default:
			panic("wrong type order msg")
		}
	}
}

func (order *order) start_ctrbcs(input []byte, output chan pb.RBCOut, msgIn []chan pb.RBCMsg, done chan bool) {
	for i := 0; i < order.Node_num; i++ {
		if i == order.ID-1 {
			//start ctrbc leader
			newBcl := rbc.NewBroadcast_leader(i+1, i+1, 1, order.Node_num, input, output, msgIn[i], order.msgoutCH, order.Round, done)
			go newBcl.Start()
		} else {
			//start ctrbc follower
			newBcf := rbc.NewBroadcast_follower(order.ID, i+1, 1, order.Node_num, output, msgIn[i], order.msgoutCH, order.Round, done)
			go newBcf.Start()
		}

	}
}

func (order *order) handle_msgOutCH() {
	for {
		msg := <-order.msgoutCH
		//fmt.Println("get a msg inside handle_msgOutCH")
		if msg.ID > order.Node_num || msg.ID <= 0 {
			panic("wrong msg ID")
		}
	
		if msg.ID == order.ID {
			continue
		}
		//order.conMsgBuff[msg.ID-1] <- msg
		select {
		case order.conMsgBuff[msg.ID-1] <- msg:
		default:
			fmt.Println("send buffer full ", msg.ID)
			order.conMsgBuff[msg.ID-1] <- msg
		}

	}
}

func (order *order) handle_conMsg(id int) {
	dropflag := false
	waitcount := 0
	for {
		//get msg

		msg := <-order.conMsgBuff[id-1]

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
				err := order.net.Direct_Send(order.orderCons[id-1], msgbyte)
				if err != nil {
					for {
						con, err := network.Dial(order.ips[id-1])
						fmt.Println("dial:", order.ips[id-1])
						if err != nil {
							time.Sleep(time.Duration(1) * time.Second)
							waitcount++
							if waitcount > 1 {
								dropflag = true
								break
							}
						} else {
							order.orderCons[id-1] = con
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

func checkbabuf(old chan pb.BAMsg, new chan pb.BAMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.MVBARound >= int32(height) {
			new <- oldmsg
		}
	}
}
func checkrbcbuf(old chan pb.RBCMsg, new chan pb.RBCMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.Round >= int32(height) {
			new <- oldmsg
		}
	}
}

func byteArrayToBoolArray(arr []byte, size int) []bool {
	result := make([]bool, size)
	for i, b := range arr {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}
	return result
}

func boolArrayToByteArray(arr []bool) []byte {
	size := (len(arr) + 7) / 8
	result := make([]byte, size)
	for i, b := range arr {
		byteIndex := i / 8
		bitIndex := uint(i % 8)
		if b {
			result[byteIndex] |= 1 << bitIndex
		}
	}
	return result
}

func check_input_rbc(input []byte, mine []bool, done chan bool) bool {
	result := make([]bool, len(mine))
	for i, b := range input {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}

	for {
		isvalid := true
		select {
		case <-done:
			return false
		default:
		}
		for i, b := range result {
			if b && !mine[i] {
				isvalid = false
				break
			}
		}
		if isvalid {
			return true
		} else {
			time.Sleep(time.Millisecond * 200)
		}
	}
}

func (order *order) wait_rbc_done(input []byte, mine []bool) bool {
	result := make([]bool, len(mine))
	for i, b := range input {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= len(result) {
				break
			}
			result[index] = (b >> j & 1) == 1
		}
	}

	for {
		isvalid := true
		for i, b := range result {
			if b && !mine[i] {
				isvalid = false
				break
			}
		}
		if isvalid {
			return true
		} else {
			time.Sleep(time.Millisecond * 50)
		}
	}
}

func (order *order) handle_signal2tp() {
	for {
		signal := <-order.signal2tpCH
		err := order.net.Send(order.tpCon, signal)
		if err != nil {
			panic(err)
		}

	}
}

