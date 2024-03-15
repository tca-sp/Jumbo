package broadcast

import (
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"sync"
)

type SendMsg struct {
	ID      int
	Type    int //0 for protocol msg.  1 for recovery msg. 2 for callhelp msg
	Content []byte
	Height  int //use for recovery msg, help to reduce buffer of network
}

type BC_l struct {
	id           int //node id
	num          int //node number
	sigmeta      cy.Signature
	input        chan []byte
	output       chan pb.BCBlock
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
}

type BC_f struct {
	id           int
	num          int
	threshold    int
	sigmeta      cy.Signature
	futurebuffer futurebuffer //buffer legal futureblock
	output       chan pb.BCBlock
	msgIn        chan []byte
	msgOut       chan SendMsg
	bcCH         chan pb.BCMsg
	height       int
	lastblkID    []byte
	lastblock    pb.BCBlock
	signs        [][]byte
}

type futurebuffer struct {
	lowwest  int
	lock     *sync.Mutex
	bcblocks []pb.BCBlock
}
