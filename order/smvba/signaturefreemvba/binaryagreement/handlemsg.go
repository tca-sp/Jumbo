package binaryagreement

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	pb "dumbo_fabric/struct"
	"encoding/binary"
)

func (ba *BA) handle_loopmsg(loopinfo *LoopInfo, oldbuf chan pb.BAMsg, newbuf chan pb.BAMsg) {

	for {
		var msg pb.BAMsg
		select {
		case <-ba.close:
			return
		case <-loopinfo.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case <-loopinfo.close:
				return
			case msg = <-oldbuf:
			default:
				select {
				case <-ba.close:
					return
				case <-loopinfo.close:
					return
				case msg = <-ba.msginCH:
				case msg = <-oldbuf:
				}
			}
		}

		if msg.MVBARound != int32(loopinfo.mvbaround) {
			//get an old round msg
			panic("get a msg from unexpected MVBARound")
		}
		if msg.BARound != int32(loopinfo.baround) {
			//get an old round msg
			panic("get a msg from unexpected BARound")
		}
		if msg.Type == 4 {
			//get a finish msg
			//fmt.Println("BA:get a finish msg of height", msg.MVBARound, "from", msg.ID, "of value", msg.Value)
			ba.finishCH <- msg
			continue
		} else if msg.Loop < int32(loopinfo.loop) {
			continue
		} else if msg.Loop > int32(loopinfo.loop) {
			newbuf <- msg
			continue
		}

		switch msg.Type {
		case 1:
			//get a val msg
			//fmt.Println("BA:get a val msg of height", msg.MVBARound, "of loop", loopinfo.loop, "from", msg.ID, "of value", msg.Value)
			loopinfo.valCH <- msg
		case 2:
			//get a aux msg
			//fmt.Println("BA:get a aux msg of height", msg.MVBARound, "of loop", loopinfo.loop, "from", msg.ID, "of value", msg.Value)
			loopinfo.auxCH <- msg
		case 3:
			//get a conf msg
			//fmt.Println("BA:get a conf msg of height", msg.MVBARound, "of loop", loopinfo.loop, "from", msg.ID, "of value", msg.ConfValue)
			loopinfo.confCH <- msg

		default:
			panic("get a wrong type of BA msg")

		}

	}
}

func (ba *BA) handle_val(loopinfo *LoopInfo) {

	v0count := 0
	v1count := 0

	isauxsent := false
	for {
		var valmsg pb.BAMsg
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case valmsg = <-loopinfo.valCH:
			}
		}

		if valmsg.Loop == int32(loopinfo.loop) {
			if !valmsg.Value {
				v0count++
				if v0count == ba.faults+1 {
					loopinfo.sendvalsignal <- false
				} else if v0count == ba.faults*2+1 {
					loopinfo.value0ready <- true
					loopinfo.value0ready_conf <- true
					loopinfo.values[0] = true
					if !isauxsent {
						loopinfo.sendauxsignal <- false
						isauxsent = true
					}
				}
			} else {
				v1count++
				if v1count == ba.faults+1 {
					loopinfo.sendvalsignal <- true
				} else if v1count == ba.faults*2+1 {
					loopinfo.value1ready <- true
					loopinfo.value1ready_conf <- true
					loopinfo.values[1] = true
					if !isauxsent {
						loopinfo.sendauxsignal <- true
						isauxsent = true
					}
				}
			}
		}
	}
}

func (ba *BA) handle_aux(loopinfo *LoopInfo) {
	aux0count := 0
	aux1count := 0
	auxmixcount := 0

	for {
		var auxmsg pb.BAMsg
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case auxmsg = <-loopinfo.auxCH:
			}
		}
		if auxmsg.Loop == int32(loopinfo.loop) {
			if !auxmsg.Value {
				aux0count++
				auxmixcount++
				if aux0count == ba.faults*2+1 {
					loopinfo.aux0ready <- true
				}
				if auxmixcount == ba.faults*2+1 {
					loopinfo.auxmixready <- true
				}
			} else {
				aux1count++
				auxmixcount++
				if aux1count == ba.faults*2+1 {
					loopinfo.aux1ready <- true
				}
				if auxmixcount == ba.faults*2+1 {
					loopinfo.auxmixready <- true
				}
			}

		}

	}
}

func (ba *BA) handle_conf(loopinfo *LoopInfo) {
	conf0count := 0
	conf1count := 0
	confmixcount := 0

	for {
		var confmsg pb.BAMsg
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case confmsg = <-loopinfo.confCH:
			}
		}

		if confmsg.Loop == int32(loopinfo.loop) {
			if confmsg.ConfValue[0] && confmsg.ConfValue[1] {
				confmixcount++
				if confmixcount == ba.faults*2+1 {
					loopinfo.confmixready <- true
				}
			} else if confmsg.ConfValue[0] && !confmsg.ConfValue[1] {
				conf0count++
				confmixcount++
				if confmixcount == ba.faults*2+1 {
					loopinfo.confmixready <- true
				}
				if conf0count == ba.faults*2+1 {
					loopinfo.conf0ready <- true
				}
			} else if !confmsg.ConfValue[0] && confmsg.ConfValue[1] {
				conf1count++
				confmixcount++
				if confmixcount == ba.faults*2+1 {
					loopinfo.confmixready <- true
				}
				if conf1count == ba.faults*2+1 {
					loopinfo.conf1ready <- true
				}
			}

		}

	}

}

func (ba *BA) handle_finish() {
	//fmt.Println("BA:inside handle finish")
	finish0count := 0
	finish1count := 0
	for {
		var finishmsg pb.BAMsg
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case finishmsg = <-ba.finishCH:
			}
		}
	//	fmt.Println("BA:handle finish msg from ", finishmsg.ID, "of value", finishmsg.Value)
		if !finishmsg.Value {
			finish0count++
			if finish0count == ba.faults+1 {
				ba.sendfinishsignal <- false
			} else if finish0count == ba.faults*2+1 {
				ba.outputCH <- false
				close(ba.close)
				fmt.Println("BA:done ba with output", false)
				return
			}
		} else {
			finish1count++
			if finish1count == ba.faults+1 {
				ba.sendfinishsignal <- true
			} else if finish1count == ba.faults*2+1 {
				ba.outputCH <- true
				close(ba.close)
				fmt.Println("BA:done ba with output", true)
				return
			}
		}

	}
}

func (ba *BA) handle_coin(loopinfo *LoopInfo) bool {
	if loopinfo.loop == 0 {
		return true
	}
	//fake coin
	h := sha256.New()
	h.Write(IntToBytes(ba.loop + ba.mvbaround*100 + ba.baround*10 + loopinfo.loop))
	hash := h.Sum(nil)

	coin := int(hash[0])

	if coin%2 == 0 {
		return true
	} else {
		return false
	}
}

func IntToBytes(n int) []byte {
	x := int32(n)
	bytesBuffer := bytes.NewBuffer([]byte{})
	binary.Write(bytesBuffer, binary.BigEndian, x)
	return bytesBuffer.Bytes()
}

