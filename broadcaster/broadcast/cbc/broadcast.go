package broadcast

import (
	cy "dumbo_fabric/crypto/signature"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"sync"

	mapset "github.com/deckarep/golang-set"
)

type SendMsg struct {
	ID      int
	Type    int //0 for protocol msg.  1 for recovery msg. 2 for callhelp msg
	Content []byte
	Height  int //use for recovery msg, help to reduce buffer of network
}

type BC_l struct {
	nid          int //node id
	lid          int //leader id
	sid          int //sub-broadcast instance
	num          int //node number
	sigmeta      cy.Signature
	input        chan []byte
	output       chan []byte
	txbuff       [][]byte
	msgIn        chan []byte
	msgOut       chan SendMsg
	height       int
	lastblkID    []byte
	lastblock    pb.BCBlock
	lastsigns    []byte
	lastsignmems []int32
	lastsignbyte []byte
	threshold    int
	batchsize    int
	db           *leveldb.DB
	callhelpCH   chan pb.CallHelp
	testmode     bool
	signal2tpCH  chan []byte
}

type BC_f struct {
	nid            int
	lid            int
	sid            int
	num            int
	threshold      int
	db             *leveldb.DB
	sigmeta        cy.Signature
	callhelp       callhelp       //info of mine callhelp
	callhelpbuffer callhelpbuffer //buffer callhelp msgs from others
	futurebuffer   futurebuffer   //buffer legal futureblock
	output         chan []byte
	msgIn          chan []byte
	msgOut         chan SendMsg
	bcCH           chan pb.BCMsg
	callhelpCH     chan pb.CallHelp //buffer callhelp msg from others
	helpCH         chan pb.BCMsg    //buffer help msg from others
	height         int
	lastblkID      []byte
	lastblock      pb.BCBlock
	signs          [][]byte
	testmode       bool
}

type callhelpbuffer struct {
	lock   *sync.Mutex
	buffer map[int]mapset.Set[int]
}

type callhelp struct {
	iscallhelp bool
	highest    int //the highest block that has been called help
	lock       *sync.Mutex
	missblocks mapset.Set[int]
}

type futurebuffer struct {
	lowwest  int
	lock     *sync.Mutex
	bcblocks []pb.BCBlock
}
