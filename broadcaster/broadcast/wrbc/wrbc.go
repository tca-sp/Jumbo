package wrbc

import (
	"bytes"
	"dumbo_fabric/database/leveldb"
	pb "dumbo_fabric/struct"
	"encoding/binary"
	"log"
)

type SendMsg struct {
	ID      int
	Type    int //0 for protocol msg.  1 for recovery msg. 2 for callhelp msg
	Content []byte
	Height  int //use for recovery msg, help to reduce buffer of network
}

type BC_l struct {
	nid       int //node id
	lid       int //leader id
	sid       int //sub-broadcast instance
	num       int //node number
	threshold int
	height    int
	batchsize int

	input  chan []byte //input of broadcast
	output chan []byte //output of broadcast
	txbuff [][]byte
	msgIn  chan []byte  //protocol msg received
	msgOut chan SendMsg //protocol msg sending to others

	readyCH chan pb.WRBCMsg //store ready msg
	echoCH  chan pb.WRBCMsg //store echo msg

	callhelpCH chan pb.WRBCMsg

	log         log.Logger
	db          leveldb.DB
	testmode    bool
	signal2tpCH chan []byte
}

type BC_f struct {
	nid       int
	lid       int
	sid       int
	num       int
	threshold int
	height    int

	output chan []byte
	msgIn  chan []byte
	msgOut chan SendMsg

	valCH   chan pb.WRBCMsg //store val msg
	readyCH chan pb.WRBCMsg //store ready msg
	echoCH  chan pb.WRBCMsg //store echo msg

	roundinfoCH chan Roundinfo

	callhelpCH chan pb.WRBCMsg
	helpCH     chan pb.WRBCMsg

	log      log.Logger
	db       leveldb.DB
	testmode bool
}

type Output struct {
	Height   int
	Hasvalue bool
	Value    []byte
	Hash     []byte
}

type Roundinfo struct {
	height    int32
	valuehash []byte
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
