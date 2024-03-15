package main

import (
	pb "dumbo_fabric/struct"
	"sync"

	mapset "github.com/deckarep/golang-set"
)

type sendmsg struct {
	id      int
	msgtype int //1: protomsg 2:callhelp 3:help
	content []byte
	key     []byte //key for database, use only in recovermsg
}

type cut struct {
	old [][]int
	new [][]int
}

//channel_status
type cs struct {
	channelID string
	number    int
	pre_hash  []byte
}

type key struct {
	lid    int32
	sid    int32
	height int32
}

type callhelpbuffer struct {
	lock       *sync.Mutex
	missblocks map[key]mapset.Set[int32] //indexs: 1:lid 2:sid 3:height value: id
}

type Oldblocks struct {
	lock   *sync.Mutex
	blocks [][][]pb.BCBlock
}
