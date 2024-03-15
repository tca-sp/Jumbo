package binaryagreement

import (
	pb "dumbo_fabric/struct"
	"fmt"

	"github.com/golang/protobuf/proto"
)

func NewBA(num int, id int, mvbaround int, baround int, input bool, secondinput chan bool, outputCH chan bool, msgin chan pb.BAMsg, msgout chan pb.SendMsg) *BA {

	return &BA{
		num:    num,
		id:     id,
		faults: (num - 1) / 3,

		mvbaround: mvbaround,
		baround:   baround,
		loop:      0,

		input:       input,
		secondinput: secondinput,
		outputCH:    outputCH,

		msginCH:  msgin,
		msgoutCH: msgout,

		finishCH: make(chan pb.BAMsg, 1000),
		close:    make(chan bool),

		sendfinishsignal: make(chan bool, 3),
	}

}

func (ba *BA) Launch() {
	fmt.Println("start biased ba")
	//get input
	input := ba.input

	//handle finish
	go ba.handle_finish()

	var oldloopbuf chan pb.BAMsg
	var newloopbuf chan pb.BAMsg
	oldloopbuf = make(chan pb.BAMsg, 2000)
	newloopbuf = make(chan pb.BAMsg, 2000)
	for {

		loopinfo := &LoopInfo{
			mvbaround: ba.mvbaround,
			baround:   ba.baround,
			loop:      ba.loop,
			input:     input,
			values:    make([]bool, 2),
			valCH:     make(chan pb.BAMsg, 1000),
			auxCH:     make(chan pb.BAMsg, 1000),
			confCH:    make(chan pb.BAMsg, 1000),

			value0ready:   make(chan bool, 10),
			value1ready:   make(chan bool, 10),
			sendvalsignal: make(chan bool, 10),

			aux0ready:     make(chan bool, 10),
			aux1ready:     make(chan bool, 10),
			auxmixready:   make(chan bool, 10),
			sendauxsignal: make(chan bool, 10),

			conf0ready:   make(chan bool, 10),
			conf1ready:   make(chan bool, 10),
			confmixready: make(chan bool, 10),

			value0ready_conf: make(chan bool, 10),
			value1ready_conf: make(chan bool, 10),

			close: make(chan bool),
		}
		if loopinfo.loop == 0 {
			loopinfo.secondinput = ba.secondinput
		}

		loopinfo.values[0] = false
		loopinfo.values[1] = false

		//to be done:check oldbuf
		checkbuf(oldloopbuf, newloopbuf, loopinfo.loop)
		oldloopbuf = newloopbuf
		newloopbuf = make(chan pb.BAMsg, 1000)

		go ba.handle_loopmsg(loopinfo, oldloopbuf, newloopbuf)
		//send val msg
		go ba.sendval(loopinfo)
		loopinfo.sendvalsignal <- input

		go ba.sendaux(loopinfo)

		go ba.sendfinish(loopinfo)
		//handle val msg
		go ba.handle_val(loopinfo)
		//handle aux msg
		go ba.handle_aux(loopinfo)
		//handle conf msg
		go ba.handle_conf(loopinfo)
		//send conf
		v0_conf, v1_conf, a0, a1, amix := false, false, false, false, false
		for {

			confready := false
			select {
			case <-ba.close:
				return
			case <-loopinfo.value0ready:
				v0_conf = true
				if a0 || (v1_conf && amix) {
					confready = true
				}
			case <-loopinfo.value1ready:
				v1_conf = true
				if a1 || (v0_conf && amix) {
					confready = true
				}
			case <-loopinfo.aux0ready:
				a0 = true
				if v0_conf {
					confready = true
				}
			case <-loopinfo.aux1ready:
				a1 = true
				if v1_conf {
					confready = true
				}
			case <-loopinfo.auxmixready:
				amix = true
				if v0_conf && v1_conf {
					confready = true
				}
			}
			//fmt.Println("tmp conf signal of loop ", loopinfo.loop, ":", v0_conf, v1_conf, a0, a1, amix)
			if confready {
				break
			}

		}
		fmt.Println("conf ready")
		go ba.sendconf(loopinfo)
		v0, v1, c0, c1, cmix := false, false, false, false, false
		//coin
		for {

			coinready := false
			select {
			case <-ba.close:
				return
			case <-loopinfo.value0ready_conf:
				v0 = true
				if c0 || (v1 && cmix) {
					coinready = true
				}
			case <-loopinfo.value1ready_conf:
				v1 = true
				if c1 || (v0 && cmix) {
					coinready = true
				}
			case <-loopinfo.conf0ready:
				c0 = true
				if v0 {
					coinready = true
				}
			case <-loopinfo.conf1ready:
				c1 = true
				if v1 {
					coinready = true
				}
			case <-loopinfo.confmixready:
				cmix = true
				if v0 && v1 {
					coinready = true
				}
			}
			if coinready {
				break
			}

		}
		fmt.Println("coin ready")
		coin := ba.handle_coin(loopinfo)
		fmt.Println("get coin of loop", loopinfo.loop, coin)
		if loopinfo.values[0] && loopinfo.values[1] {
			fmt.Println("failed to commit for mutil values, start next loop")
			input = coin
			ba.loop++
			close(loopinfo.close)
		} else {
			if loopinfo.values[0] && !coin {
				ba.sendfinishsignal <- false
			} else if loopinfo.values[1] && coin {
				ba.sendfinishsignal <- true
			} else {
				fmt.Println("failed to commit for miss coin, start next loop")
				if loopinfo.values[0] {
					input = false
				} else {
					input = true
				}
				ba.loop++
				close(loopinfo.close)
				continue
			}
			<-ba.close
			return
		}

	}

}

func (ba *BA) sendval(loopinfo *LoopInfo) {
	var val bool
	has0sent := false
	has1sent := false
	for {
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case val = <-loopinfo.sendvalsignal:
			}
		}

		if val && has1sent {
			continue
		}
		if !val && has0sent {
			continue
		}
		//fmt.Println("BA: send val msg of loop", loopinfo.loop, " of", val, "with msgout buff length ", len(ba.msgoutCH))
		if loopinfo.loop == 0 && !val {
			//wait for second input
			go ba.sendsencondval(loopinfo, true)
		}
		valmsg := pb.BAMsg{
			ID:        int32(ba.id),
			MVBARound: int32(loopinfo.mvbaround),
			BARound:   int32(loopinfo.baround),
			Loop:      int32(loopinfo.loop),
			Type:      1,
			Value:     val,
		}
		msgbytes, err := proto.Marshal(&valmsg)
		if err != nil {
			panic(err)
		}
		for i := 1; i <= ba.num; i++ {
			if i == ba.id {
				loopinfo.valCH <- valmsg
			} else {
				ba.msgoutCH <- pb.SendMsg{ID: i, Type: 5, Msg: msgbytes}
			}
		}
		if val && loopinfo.loop == 0 {
			loopinfo.sendauxsignal <- true
			loopinfo.values[1] = true
		}
		//fmt.Println("BA:done send val msg of loop", loopinfo.loop, " of", val)
		if val {
			has1sent = true
		} else {
			has0sent = true
		}

		if has0sent && has1sent {
			return
		}
	}

}

func (ba *BA) sendsencondval(loopinfo *LoopInfo, val bool) {
	select {
	case <-ba.close:
		return
	default:
		select {
		case <-ba.close:
			return
		case <-loopinfo.secondinput:
		}
	}
	//fmt.Println("BA: send 2nd val msg of loop", loopinfo.loop, "of", val, "with msgout buff length ", len(ba.msgoutCH))
	valmsg := pb.BAMsg{
		ID:        int32(ba.id),
		MVBARound: int32(loopinfo.mvbaround),
		BARound:   int32(loopinfo.baround),
		Loop:      int32(loopinfo.loop),
		Type:      1,
		Value:     val,
	}
	msgbytes, err := proto.Marshal(&valmsg)
	if err != nil {
		panic(err)
	}
	for i := 1; i <= ba.num; i++ {
		if i == ba.id {
			loopinfo.valCH <- valmsg
		} else {
			ba.msgoutCH <- pb.SendMsg{ID: i, Type: 5, Msg: msgbytes}
		}
	}
	loopinfo.sendauxsignal <- true
	loopinfo.values[1] = true

	//fmt.Println("BA:done send 2nd val msg of loop", loopinfo.loop, "of", val)
}

func (ba *BA) sendaux(loopinfo *LoopInfo) {
	has1sent := false
	has0sent := false
	var val bool
	for {
		select {
		case <-ba.close:
			return
		default:
			select {
			case <-ba.close:
				return
			case val = <-loopinfo.sendauxsignal:
			}
		}
		if val && has1sent {
			continue
		} else if !val && has0sent {
			continue
		}
		if val {
			has1sent = true
		} else {
			has0sent = true
		}
		//fmt.Println("BA: send aux msg of loop", loopinfo.loop, "of", val, "with msgout buff length ", len(ba.msgoutCH))
		auxmsg := pb.BAMsg{
			ID:        int32(ba.id),
			MVBARound: int32(loopinfo.mvbaround),
			BARound:   int32(loopinfo.baround),
			Loop:      int32(loopinfo.loop),
			Type:      2,
			Value:     val,
		}
		msgbytes, err := proto.Marshal(&auxmsg)
		if err != nil {
			panic(err)
		}
		for i := 1; i <= ba.num; i++ {
			if i == ba.id {
				loopinfo.auxCH <- auxmsg
			} else {
				ba.msgoutCH <- pb.SendMsg{ID: i, Type: 5, Msg: msgbytes}
			}
		}
		if has0sent && has1sent {
			return
		}

		//fmt.Println("BA:done send aux msg of loop", loopinfo.loop, "of", val)
	}

}

func (ba *BA) sendconf(loopinfo *LoopInfo) {
	//fmt.Println("BA: send conf msg of loop", loopinfo.loop, "of", loopinfo.values, "with msgout buff length ", len(ba.msgoutCH))
	confmsg := pb.BAMsg{
		ID:        int32(ba.id),
		MVBARound: int32(loopinfo.mvbaround),
		BARound:   int32(loopinfo.baround),
		Loop:      int32(loopinfo.loop),
		Type:      3,
		ConfValue: loopinfo.values,
	}
	msgbytes, err := proto.Marshal(&confmsg)
	if err != nil {
		panic(err)
	}
	for i := 1; i <= ba.num; i++ {
		if i == ba.id {
			loopinfo.confCH <- confmsg
		} else {
			ba.msgoutCH <- pb.SendMsg{ID: i, Type: 5, Msg: msgbytes}
		}
	}

	//fmt.Println("BA:done send conf msg of loop", loopinfo.loop, "of", loopinfo.values)
}

func (ba *BA) sendfinish(loopinfo *LoopInfo) {
	var val bool
	select {
	case <-ba.close:
		return
	default:
		select {
		case <-ba.close:
			return
		case val = <-ba.sendfinishsignal:
		}
	}
	//fmt.Println("BA: send finish msg of loop", loopinfo.loop, "of", val, "with msgout buff length ", len(ba.msgoutCH))
	finishmsg := pb.BAMsg{
		ID:        int32(ba.id),
		MVBARound: int32(loopinfo.mvbaround),
		BARound:   int32(loopinfo.baround),
		Type:      4,
		Value:     val,
	}
	msgbytes, err := proto.Marshal(&finishmsg)
	if err != nil {
		panic(err)
	}
	for i := 1; i <= ba.num; i++ {
		if i == ba.id {
			ba.finishCH <- finishmsg
		} else {
			ba.msgoutCH <- pb.SendMsg{ID: i, Type: 5, Msg: msgbytes}
		}
	}
	//fmt.Println("BA:done send finish msg of loop", loopinfo.loop, "of", val)
}

func checkbuf(old chan pb.BAMsg, new chan pb.BAMsg, height int) {
	length := len(old)
	for i := 0; i < length; i++ {
		oldmsg := <-old
		if oldmsg.Loop >= int32(height) {
			new <- oldmsg
		}
	}
}

