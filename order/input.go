package main

import (
	"bytes"
	"crypto/sha256"
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
	"fmt"
	"math/rand"
	"reflect"
	"time"

	"github.com/golang/protobuf/proto"
)

func (od_m *order_m) check_send() {

	//fmt.Println(heights)
	res := check_input(od_m.hs, od_m.lastCommit, od_m.Node_num, od_m.K)
	//fmt.Println("******check input", heights, od_m.lastCommit)
	if res {
		//generate msg sending to order
		time.Sleep(time.Millisecond * 100)
		var input []byte

		input = od_m.generate_input()

		od_m.sendIntputCH <- input
		od_m.Round++
		fmt.Println(time.Now(), "generate a new input")
		od_m.ready = false
		time.Sleep(time.Duration(1) * time.Millisecond)
	} else {
		od_m.ready = true
	}

}

func (od_m *order_m) generate_input_QCagg() []byte {
	newQCagg := pb.QCaggProof{
		MessageHash: make([][]byte, od_m.Node_num),
		Heights:     make([]int32, od_m.Node_num),
		Mems:        make([][]byte, od_m.Node_num),
	}

	var tmpqcs [][]byte
	var tmpmsgs [][]byte
	var tmpmems [][]int32

	var tmpheights [][]pb.BCBlock
	tmpheights = od_m.heights

	for i, blks := range tmpheights {	
		if blks[0].RawBC.Height >= 0 {

//			height1 := blks[0].RawBC.Height

			hash := sha256.New()
			hashMsg, _ := proto.Marshal(blks[0].RawBC)
			hash.Write(hashMsg)
			blkID := hash.Sum(nil)[:]
			signbyte := append(blkID, IntToBytes(int(blks[0].RawBC.Height))...)
			signbyte = append(signbyte, IntToBytes(int(blks[0].RawBC.Leader))...)
			signbyte = append(signbyte, IntToBytes(int(1))...)

			//verify QC
//			if !od_m.sigmeta.BatchSignatureVerify(blks[0].BatchSigns.Signs, signbyte, blks[0].BatchSigns.Mems) {
//				panic("wrong input QC")
//			}

			newQCagg.MessageHash[i] = blkID
			newQCagg.Heights[i] = blks[0].RawBC.Height
			newQCagg.Mems[i] = int32s2bytes(blks[0].BatchSigns.Mems, od_m.Node_num)

			tmpqcs = append(tmpqcs, blks[0].BatchSigns.Signs)
			tmpmsgs = append(tmpmsgs, signbyte)
			tmpmems = append(tmpmems, blks[0].BatchSigns.Mems)

//			height2 := blks[0].RawBC.Height

//			if height1 != height2 {
//				fmt.Println("data has been changed")
//			}

		} else {
			newQCagg.Heights[i] = -1
		}

	}

	QCaggsign := od_m.sigmeta.QCAggregate(tmpqcs, tmpmsgs, tmpmems)
	newQCagg.AggQC = QCaggsign

//	if od_m.Round == 0 {
//		fmt.Println("signature:", QCaggsign)
//		fmt.Println("msgs:", tmpmsgs)
//		fmt.Println("mems:", tmpmems)
//	}

//	if !od_m.sigmeta.QCAggregateVerify(QCaggsign, tmpmsgs, tmpmems) {
//		fmt.Println("qcs:",tmpqcs)
//		fmt.Println("signature:", QCaggsign)
//		fmt.Println("msgs:", tmpmsgs)
//		fmt.Println("mems:", tmpmems)
//		panic("wrong input QCagg")
//	}

	newinput, err := proto.Marshal(&newQCagg)
	if err != nil {
		panic(err)
	} else {
		return newinput
	}
}

func check_inputs_QCagg(new []byte, old []byte, num int, sigmeta cy.Signature) bool {
	if len(old) == 0 {
		return true
	}
	newQCagg := pb.QCaggProof{}
	err := proto.Unmarshal(new, &newQCagg)
	if err != nil {
		panic(err)
	}

	oldQCagg := pb.QCaggProof{}
	err = proto.Unmarshal(old, &oldQCagg)
	if err != nil {
		panic(err)
	}

	//check plaintext
	count := 0
	for i := 0; i < num; i++ {
		if newQCagg.Heights[i] > oldQCagg.Heights[i] {
			count++
		} else if newQCagg.Heights[i] < oldQCagg.Heights[i] {
			fmt.Println("wrong heights")
			return false
		}
	}
	if count <= num*2/3 {
		fmt.Println("not enough heights")
		return false
	}

	//check QCagg signature
	//recover msghash and mems
	var tmpmsgs [][]byte
	var tmpmems [][]int32
	for i := 0; i < num; i++ {
		if newQCagg.Heights[i] >= 0 {
			signbyte := append(newQCagg.MessageHash[i], IntToBytes(int(newQCagg.Heights[i]))...)
			signbyte = append(signbyte, IntToBytes(i+1)...)
			signbyte = append(signbyte, IntToBytes(int(1))...)
			tmpmsgs = append(tmpmsgs, signbyte)
			tmpmems = append(tmpmems, bytes2int32s(newQCagg.Mems[i], num))
		}

	}

	return sigmeta.QCAggregateVerify(newQCagg.AggQC, tmpmsgs, tmpmems)

}

func (od_m *order_m) generate_input() []byte {
	
	od_m.oldBCBlocks.lock.Lock()
	defer od_m.oldBCBlocks.lock.Unlock()
	if od_m.BroadcastType == "CBC_QCagg" {
		return od_m.generate_input_QCagg()
	}
	input := make([]*pb.HighProof, od_m.Node_num*od_m.K)
	input_rbc := make([]int32, od_m.Node_num*od_m.K)
	
	th := od_m.Node_num * 2 / 3

	if od_m.ID == 1 && od_m.ByzFairness != 0 && od_m.BroadcastType == "WRBC" {
		rand.Seed(time.Now().UnixNano())
		r:=rand.Intn(od_m.ByzFairness)
		if r != 1 {
			fmt.Println("attack")
			count:=0
			signal:=false
			for i, blks := range od_m.heights {
				if signal {
					input_rbc[i] = int32(od_m.lastCommit[i][0])
				} else if int(blks[0].RawBC.Height) > od_m.lastCommit[i][0] {
					input_rbc[i] = int32(od_m.lastCommit[i][0]) + 1
					count++
				} else {
					input_rbc[i] = int32(od_m.lastCommit[i][0])
				}
				if count > th {
					signal = true
				}
				//fmt.Println(blk)
				//for j, blk := range blks {
				//if  i+1 > od_m.Node_num-th{
				//	input_rbc[i] = int32(od_m.lastCommit[i][0]) + 1
				//} else {
			//		input_rbc[i] = int32(od_m.lastCommit[i][0])
			//	}

				//}
			}
		}else {
			for i, blks := range od_m.heights {
				//fmt.Println(blk)
				for j, blk := range blks {
					if !reflect.DeepEqual(blk, pb.BCBlock{}) {

						input_rbc[j*od_m.Node_num+i] = blk.RawBC.Height

					}

				}
			}
		}
	}else{

	for i, blks := range od_m.heights {
		//fmt.Println(blk)
		for j, blk := range blks {
			if !reflect.DeepEqual(blk, pb.BCBlock{}) {
				switch od_m.BroadcastType {
				case "RBC":
					input_rbc[j*od_m.Node_num+i] = blk.RawBC.Height
				case "WRBC":
					input_rbc[j*od_m.Node_num+i] = blk.RawBC.Height

				case "CBC":
					input[j*od_m.Node_num+i] = &pb.HighProof{
						RawBC:      blk.RawBC,
						Sign:       blk.Sign,
						BatchSigns: blk.BatchSigns,
					}
				default:
					panic("wrong broadcast type")
				}

			}

		}
	}
}
	switch od_m.BroadcastType {
	case "RBC":
		inputs := &pb.HeightRBC{
			Heights: input_rbc,
		}
		inputsByte, _ := proto.Marshal(inputs)
		fmt.Println("generate a new input in generate_input of", input_rbc)
		return inputsByte
	case "WRBC":
		inputs := &pb.HeightRBC{
			Heights: input_rbc,
		}
		inputsByte, _ := proto.Marshal(inputs)
		fmt.Println("generate a new input in generate_input of", input_rbc)
		return inputsByte
	case "CBC":
		inputs := &pb.HighProofs{
			HPs: input[:],
		}
		inputsByte, _ := proto.Marshal(inputs)
		return inputsByte
	default:
		panic("wrong broadcast type")
	}

	//fmt.Println("************", input[0].Rawblk.Height, input[1].Rawblk.Height, input[2].Rawblk.Height, input[3].Rawblk.Height)

}

func check_inputs(new []byte, old []byte, num int, sigmeta cy.Signature, oldblocks [][][]pb.BCBlock) bool {
	if len(old) == 0 {
		return true
	}

	oldProofs := pb.HighProofs{}
	newProofs := pb.HighProofs{}

	err := proto.Unmarshal(old, &oldProofs)
	if err != nil {
		panic(err)
	}

	err = proto.Unmarshal(new, &newProofs)
	if err != nil {
		panic(err)
	}

	if len(oldProofs.HPs) != num || len(newProofs.HPs) != num {
		fmt.Println("check length wrong")
		return false
	} else {
		count := 0
		for i, hp := range newProofs.HPs {
			//check batch signature
			/*if hp.RawBC.Height >= 0 {
				hp.RawBC.Timestamp = 0
				hash := sha256.New()
				hashMsg, _ := proto.Marshal(hp.RawBC)
				hash.Write(hashMsg)
				blkID := hash.Sum(nil)[:]

				signbyte := append(blkID, IntToBytes(int(hp.RawBC.Height))...)
				signbyte = append(signbyte, IntToBytes(int(hp.RawBC.Leader))...)
				signbyte = append(signbyte, IntToBytes(int(hp.RawBC.K))...)
				if err != nil {
					fmt.Println(err, " marshal signbyte")
				}
				res := sigmeta.BatchSignatureVerify(hp.BatchSigns.Signs, signbyte, hp.BatchSigns.Mems)
				if !res {
					fmt.Println("wrong batchsignature")
					return false
				}
			}*/

			//check height
			diff := hp.RawBC.Height - oldProofs.HPs[i].RawBC.Height
			if diff > 0 {
				//check signature by check table first
				if len(oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1]) > int(diff) {
					target := hp.RawBC.Height - oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1][0].RawBC.Height
					if target >= 0 {
						if !bytes.Equal(hp.BatchSigns.Signs, oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1][target].BatchSigns.Signs) {
							fmt.Println("signature missmatch with received, lid", hp.RawBC.Leader, "height", hp.RawBC.Height, "my leader", oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1][diff-1].RawBC.Leader, "my height", oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1][diff-1].RawBC.Height)
							for _, bc := range oldblocks[hp.RawBC.Leader-1][hp.RawBC.K-1] {
								fmt.Println(bc.RawBC.Height)
							}
							return false
						}
					}

				} else {
					//check signature
					//hp.RawBC.Timestamp = 0
					hash := sha256.New()
					hashMsg, _ := proto.Marshal(hp.RawBC)
					hash.Write(hashMsg)
					blkID := hash.Sum(nil)[:]

					signbyte := append(blkID, IntToBytes(int(hp.RawBC.Height))...)
					signbyte = append(signbyte, IntToBytes(int(hp.RawBC.Leader))...)
					signbyte = append(signbyte, IntToBytes(int(hp.RawBC.K))...)
					if err != nil {
						fmt.Println(err, " marshal signbyte")
					}
					res := sigmeta.BatchSignatureVerify(hp.BatchSigns.Signs, signbyte, hp.BatchSigns.Mems)
					if !res {
						fmt.Println("wrong batchsignature")
						return false
					}
				}

				count++
			} else if hp.RawBC.Height < oldProofs.HPs[i].RawBC.Height {
				fmt.Println("check value wrong")
				return false
			}
		}
		/*
			for rightness checking
		*/
		if count > (num * 2 / 3) {
			//if count > 0 {
			return true
		} else {
			fmt.Println("check num wrong")
			return false
		}
	}
}

func check_input_RBC(input []byte, heightsnow [][]int32, close chan bool) bool {

	heightrbc := pb.HeightRBC{}
	err := proto.Unmarshal(input, &heightrbc)
	if err != nil {
		panic(err)
	}
	//fmt.Println("input heights:", heightrbc.Heights)
	heights := heightrbc.Heights
	num := len(heightsnow)

	for {
		select {
		case <-close:
			return false
		default:
		}
		isvalid := true
		for i := 0; i < len(heightsnow); i++ {
			for j, height := range heightsnow[i] {
				if height < heights[j*num+i] {
					//fmt.Println("input:", heights[j*num+i], " now:", height, "height:", j*num+i+1)
					isvalid = false
					break
				}
			}
			if !isvalid {
				break
			}
		}
		if isvalid {
			//fmt.Println("check_input_RBC true")
			return true
		} else {
			//fmt.Println("heights now:", heightsnow)
			time.Sleep(time.Millisecond * 200)
		}

	}
}

//check if receive enough msg to generate a input of MVBA
func check_input(a [][]int32, b [][]int, n int, K int) bool {
	count := 0
	for i := 0; i < n; i++ {
		for j := 0; j < K; j++ {
			if int(a[i][j]) < b[i][j] {
				return false
			} else if int(a[i][j]) > b[i][j] {
				count++
			}
		}
	}

	/*
		for rightness checking
	*/
	if count > (n * 2 / 3) {
		//if count > 0 {
		return true
	} else {
		return false
	}

}

func int32s2bytes(ids []int32, max int) []byte {
	tags := make([]bool, max)

	for _, id := range ids {
		tags[id-1] = true
	}

	b := make([]byte, (len(tags)+7)/8)
	for i, v := range tags {
		if v {
			b[i/8] |= 1 << uint(i%8)
		}
	}
	return b
}

func bytes2int32s(aggids []byte, max int) []int32 {
	var result []int32
	for i, b := range aggids {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= max {
				break
			}
			if (b >> j & 1) == 1 {
				result = append(result, int32(i)*8+int32(j)+1)
			}
		}
	}
	return result
}
