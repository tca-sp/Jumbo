package reedsolomon

import (
	rs "github.com/klauspost/reedsolomon"
)

type ReedSolomon struct {
	rs rs.Encoder
}

func New(threshold int, all int) ReedSolomon {
	if all < threshold {
		panic("wrong parameters: all should be bigger than threshold")
	}
	enc, err := rs.New(threshold, all-threshold)
	if err != nil {
		panic(err)
	}
	return ReedSolomon{enc}
}

func (reed ReedSolomon) Encode(msg []byte) [][]byte {
	shards, err := reed.rs.Split(msg)
	if err != nil {
		panic(err)
	}
	//fmt.Println("shards after split", shards)
	err = reed.rs.Encode(shards)
	if err != nil {
		panic(err)
	}
	//fmt.Println("shards after encode", shards)
	ok, err := reed.rs.Verify(shards)
	if !ok {
		panic(err)
	}
	return shards
}

func (reed ReedSolomon) Reconstruct(shards [][]byte, msglen int) []byte {

	err := reed.rs.Reconstruct(shards)
	if err != nil {
		panic(err)
	}
	
	ok, err := reed.rs.Verify(shards)
	if !ok {
		panic(err)
	}

	var out []byte
	for i := 0; i < msglen; i++ {
		for _, B := range shards[i] {
			out = append(out, B)
			if len(out) >= msglen {
				break
			}
		}
		if len(out) >= msglen {
			break
		}
	}
	if len(out) < msglen {
		panic("wrong length")
	}
	return out

}
