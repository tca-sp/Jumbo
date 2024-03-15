package main

import (
	"bufio"
	"dumbo_fabric/network"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v2"
)

type client struct {
	BatchSize int    `yaml:"BatchSize"`
	TxSize    int    `yaml:"TxSize"`
	TpIPPath  string `yaml:"TpIPPath"`
	//BCIPPath string `yaml:"BCIPPath"`
	//ClientIPPath   string `yaml:"ClientIPPath"`
	//ClientIP       string
	Node_num       int `yaml:"Node_num"`
	K              int `yaml:"K"`
	cons           []net.Conn
	con            net.Conn
	feedbackCH     chan []byte
	IsControlSpeed bool `yaml:"IsControlSpeed"`
	ClientNetSpeed int  `yaml:"ClientNetSpeed"`
	Sleeptime      int  `yaml:"Sleeptime"`

	PRECISION int `yaml:"PRECISION"`
}

var configpath string = "/src/dumbo_fabric/config/node.yaml"
var log1 = log.New(os.Stderr, "CLIENT: ", 3)
var islog uint64

type LogFile struct {
	f os.File
}

func (lf *LogFile) Write(p []byte) (int, error) {
	return lf.f.Write(p)
}

var logfile os.File

func main() {
	defer logfile.Close()
	var maxf = flag.Int("max", 0, "-max")
	var idf = flag.Int("id", 0, "-id")
	flag.Parse()
	max := *maxf
	id := *idf

	client := &client{}
	client.init(id)

	//msg := []byte("i'm a tx")
	msg := make([]byte, client.TxSize)
	for i := 0; i < client.TxSize; i++ {
		msg[i] = 100
	}
	net := network.New(client.IsControlSpeed, client.ClientNetSpeed, false, 0)
	net.Init()
	time.Sleep(time.Duration(client.Sleeptime+10) * time.Second)
	//go func() {
	//time.Sleep(time.Duration(10) * time.Second)

	/*timebeign := time.Now()
	amountnow := 0
	txcount := 0
	fmt.Println("start time:", time.Now())
	for {
		net.Send(client.con, msg)
		txcount++
		if txcount%10000 == 0 {
			fmt.Println("sent", txcount, "to txpool", time.Now())
		}
		elapsed := time.Since(timebeign)
		if elapsed >= time.Millisecond*time.Duration(client.PRECISION) {
			amountnow = 0
			timebeign = time.Now()
		} else {
			amountnow++
			if amountnow >= max/(1000/client.PRECISION) {
				amountnow = 0
				time.Sleep(time.Millisecond*time.Duration(client.PRECISION) - elapsed)
				timebeign = time.Now()
			}
		}
	}*/
	amountnow := 0
	var ticker time.Ticker
	firsttime := true
	for {
		net.Send(client.con, msg)
		if firsttime {
		
			fmt.Println(time.Now())
			ticker = *time.NewTicker(time.Duration(client.PRECISION) * time.Millisecond)
			firsttime = false
		}
		amountnow++
		select {
		case <-ticker.C:
			amountnow = 0
		default:
		}
		if amountnow >= max*client.PRECISION/1000 {
			<-ticker.C
			amountnow = 0
		}

	}

	//}()

	//go client.handle_feedback(id)
	//network.Listen_msg(client.ClientIP, client.feedbackCH, true)

	/*for i := 0; i < client.Node_num; i++ {
		client.cons[i].Close()
	}*/

}

func (client *client) init(id int) {
	//init log
	filepath := fmt.Sprintf("./log/log_client_%d.txt", id)

	logfile, err := os.OpenFile(filepath, os.O_RDWR|os.O_TRUNC|os.O_CREATE, 0777)
	if err != nil {
		panic(err)
	}

	logf := LogFile{f: *logfile}
	log1.SetFlags(log.Lmicroseconds)
	log1.SetOutput(&logf)

	//read config
	gopath := os.Getenv("GOPATH")
	readBytes, err := ioutil.ReadFile(gopath + configpath)
	if err != nil {
		panic(err)
	}
	err = yaml.Unmarshal(readBytes, client)
	if err != nil {
		panic(err)
	}

	//read tx_pool IP, dial and send txs to it
	TpIPPath := fmt.Sprintf("%s%sip.txt", gopath, client.TpIPPath)
	//TpIPPath := fmt.Sprintf("%s%sipm.txt", gopath, client.BCIPPath)
	fi, err := os.Open(TpIPPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br := bufio.NewReader(fi)
	var tpIP string
	a, _, c := br.ReadLine()
	if c == io.EOF {
		panic("none txpool ip")
	}
	tpIP = string(a)

	fi.Close()

	//read client IP, using to receive payback from order
	/*ClientIPPath := fmt.Sprintf("%s%sip.txt", gopath, client.ClientIPPath)
	fi, err = os.Open(ClientIPPath)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	br = bufio.NewReader(fi)
	var clientIP string
	for i := 0; i < client.Node_num; i++ {
		a, _, c := br.ReadLine()
		if c == io.EOF {
			break
		}
		if i == id-1 {
			clientIP = string(a)
		}

	}
	fi.Close()
	client.ClientIP = clientIP*/
	//listen payback msg from order

	//dial tx_pool
	con, err := network.Dial(tpIP)
	fmt.Println("dial:", tpIP)
	if err != nil {
		fmt.Println(id, " ", err)
	}
	client.con = con

	client.feedbackCH = make(chan []byte, 1000)
}

func (client *client) handle_feedback(id int) {

	flag := (id == 1)
	count := 0
	for {
		<-client.feedbackCH
		count++
		if flag {
			log1.Println("receive: ", count)
		}

	}

}

