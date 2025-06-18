package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"jumbo/network"
	"log"
	"net"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"

	pb "jumbo/client/kvstore"
	st "jumbo/struct"
)

type client struct {
	ID        int
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
	IsControlSpeed bool `yaml:"IsControlSpeed"`
	ClientNetSpeed int  `yaml:"ClientNetSpeed"`
	Sleeptime      int  `yaml:"Sleeptime"`

	PRECISION int  `yaml:"PRECISION"`
	IsLocal   bool `yaml:"IsLocal"`
	IsRealTx  bool `yaml:"IsRealTx"`
	pb.UnimplementedKeyValueStoreServer
	RealTxChan chan []byte
}

var configpath string = "/src/jumbo/config/node.yaml"
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
	time.Sleep(time.Duration(client.Sleeptime+5) * time.Second)

	amountnow := 0
	var ticker time.Ticker
	firsttime := true
	for {

		realTx := <-client.RealTxChan
		net.Send(client.con, realTx)

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

}

func (client *client) init(id int) {
	client.ID = id
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
	var tpIP string

	client.RealTxChan = make(chan []byte, 1000)
	go client.StartGrpc()

	if client.IsLocal {
		tpIP = fmt.Sprintf("127.0.0.1:%d", 11000+id)
	} else {
		TpIPPath := fmt.Sprintf("%s%sip.txt", gopath, client.TpIPPath)
		fi, err := os.Open(TpIPPath)
		if err != nil {
			fmt.Printf("Error: %s\n", err)
			return
		}

		br := bufio.NewReader(fi)

		a, _, c := br.ReadLine()
		if c == io.EOF {
			panic("none txpool ip")
		}
		tpIP = string(a)

		fi.Close()
	}

	//dial tx_pool
	con, err := network.Dial(tpIP)
	fmt.Println("dial:", tpIP)
	if err != nil {
		fmt.Println(id, " ", err)
	}
	client.con = con

}

func (client *client) StartGrpc() {
	port := fmt.Sprintf(":5000%d", client.ID)
	lis, err := net.Listen("tcp", port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterKeyValueStoreServer(s, client)
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}

func (client *client) Put(ctx context.Context, req *pb.PutRequest) (*pb.PutResponse, error) {
	key := req.Key
	val := req.Value

	realTx := st.RealTX{Key: key, Value: val}
	TxBytes, err := proto.Marshal(&realTx)
	if err != nil {
		panic(err)
	}
	client.RealTxChan <- TxBytes
	return &pb.PutResponse{Success: true}, nil
}
