package main

import "fmt"

func main() {
	max := 256
	a := []int{128, 8, 32, 2, 255, 33, 66}

	fmt.Println(a)
	b := ints2bytes(a, max)
	fmt.Println(b)
	c := bytes2ints(b, max)
	fmt.Println(c)

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
