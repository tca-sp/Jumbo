package reliablebroadcast

import (
	"bytes"
	pb "dumbo_fabric/struct"
	"encoding/binary"
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

	input        []byte         //input of broadcast
	output       chan pb.RBCOut //output of broadcast
	outtimestamp chan int64
	txbuff       [][]byte
	msgIn        chan pb.RBCMsg  //protocol msg received
	msgOut       chan pb.SendMsg //protocol msg sending to others

	readyCH chan pb.RBCMsg //store ready msg
	echoCH  chan pb.RBCMsg //store echo msg

	mypath [][]byte
	done   chan bool
}

type BC_f struct {
	nid       int
	lid       int
	sid       int
	num       int
	threshold int
	height    int

	output chan pb.RBCOut
	msgIn  chan pb.RBCMsg
	msgOut chan pb.SendMsg

	valCH   chan pb.RBCMsg //store val msg
	readyCH chan pb.RBCMsg //store ready msg
	echoCH  chan pb.RBCMsg //store echo msg

	done chan bool
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

