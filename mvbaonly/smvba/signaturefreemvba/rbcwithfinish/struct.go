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
	num       int //node number
	threshold int
	round     int

	input   []byte         //input of broadcast
	output1 chan pb.RBCOut //output by ready
	output2 chan pb.RBCOut //output by finish
	close   chan bool      //RBC can be killed by mvba

	msgIn  chan pb.RBCMsg  //protocol msg received
	msgOut chan pb.SendMsg //protocol msg sending to others

	readyCH  chan pb.RBCMsg //store ready msg
	echoCH   chan pb.RBCMsg //store echo msg
	finishCH chan pb.RBCMsg //store finish msg

	readysignal   chan bool
	output1signal chan bool
	output2signal chan bool
}

type BC_f struct {
	nid       int
	lid       int
	num       int
	threshold int
	round     int

	check_input_rbc func([]byte, [][]int32, chan bool) bool
	heights         [][]int32

	output1  chan pb.RBCOut //output by ready
	output2  chan pb.RBCOut //output by finish
	close    chan bool
	closeval chan bool

	msgIn  chan pb.RBCMsg
	msgOut chan pb.SendMsg

	valCH    chan pb.RBCMsg //store val msg
	readyCH  chan pb.RBCMsg //store ready msg
	echoCH   chan pb.RBCMsg //store echo msg
	finishCH chan pb.RBCMsg //store finish msg

	readysignal   chan []byte
	output1signal chan []byte
	output2signal chan []byte
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
