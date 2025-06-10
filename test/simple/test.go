package main

import (
	"fmt"
	"time"
)

func main() {

	t1 := time.Duration(time.Second)
	t2 := t1 / time.Duration(3)
	fmt.Println(t2)
	fmt.Println(t2.Seconds())

}

func ints2bytes(ids []int, max int) []byte {
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

func bytes2ints(aggids []byte, max int) []int {
	var result []int
	for i, b := range aggids {
		for j := uint(0); j < 8; j++ {
			index := i*8 + int(j)
			if index >= max {
				break
			}
			if (b >> j & 1) == 1 {
				result = append(result, i*8+int(j)+1)
			}
		}
	}
	return result
}
