package main

import (
	cy "dumbo_fabric/crypto/signature"
	pb "dumbo_fabric/struct"
)

func check_inputs_QCagg(new []byte, old []byte, num int, sigmeta cy.Signature) bool {
	return true
}

func check_inputs(new []byte, old []byte, num int, sigmeta cy.Signature, oldblocks [][][]pb.BCBlock) bool {
	return true
}

func check_input_RBC(input []byte, heightsnow [][]int32, close chan bool) bool {

	return true
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
