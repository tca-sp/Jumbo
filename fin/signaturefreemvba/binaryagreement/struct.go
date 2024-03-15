package binaryagreement

import (
	pb "dumbo_fabric/struct"
)

type BA struct {
	num    int
	id     int
	faults int

	mvbaround int
	baround   int
	loop      int

	input       bool
	secondinput chan bool
	outputCH    chan bool

	msginCH  chan pb.BAMsg
	msgoutCH chan pb.SendMsg

	finishCH chan pb.BAMsg
	close    chan bool

	sendfinishsignal chan bool
}

type LoopInfo struct {
	mvbaround int
	baround   int
	loop      int

	input       bool
	secondinput chan bool

	values []bool

	valCH  chan pb.BAMsg
	auxCH  chan pb.BAMsg
	confCH chan pb.BAMsg

	value0ready   chan bool
	value1ready   chan bool
	sendvalsignal chan bool

	aux0ready     chan bool
	aux1ready     chan bool
	auxmixready   chan bool
	sendauxsignal chan bool

	conf0ready   chan bool
	conf1ready   chan bool
	confmixready chan bool

	value0ready_conf chan bool
	value1ready_conf chan bool

	close chan bool
}
