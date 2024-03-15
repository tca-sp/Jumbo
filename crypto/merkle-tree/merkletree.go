package merkletree

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"

	"github.com/cbergoon/merkletree"
)

type MTContent struct {
	x []byte
}

//CalculateHash hashes the values of a TestContent
func (t MTContent) CalculateHash() ([]byte, error) {
	h := sha256.New()
	if _, err := h.Write(t.x); err != nil {
		return nil, err
	}

	return h.Sum(nil), nil
}

//Equals tests for equality of two Contents
func (t MTContent) Equals(other merkletree.Content) (bool, error) {
	_, ok := other.(MTContent)
	if !ok {
		return false, errors.New("value is not of type MTContent")
	}
	return bytes.Equal(t.x, other.(MTContent).x), nil
}

type Merkletree struct {
	merkletree.MerkleTree
}

func NewTree(shards [][]byte) (Merkletree, error) {
	//transform shards to contents
	if len(shards) <= 0 {
		panic("wrong length of input")
	}
	contents := make([]merkletree.Content, len(shards))
	for i, shard := range shards {
		contents[i] = MTContent{shard}
	}
	merkletree, err := merkletree.NewTree(contents)
	return Merkletree{*merkletree}, err

}

func (mt Merkletree) GetMerklePath(shard []byte) ([][]byte, error) {
	content := MTContent{shard}
	path, _, err := mt.MerkleTree.GetMerklePath(content)
	var pathfinal [][]byte
	pathfinal = append(pathfinal, shard)
	pathfinal = append(pathfinal, path...)
	pathfinal = append(pathfinal, mt.MerkleRoot())
	return pathfinal, err
}

//check root first
func VerifyPath(root []byte, path [][]byte, index int) bool {
	pathlen := len(path)
	if pathlen < 3 {
		fmt.Println("wrong length")
		return false
	}
	if !bytes.Equal(root, path[pathlen-1]) {
		fmt.Println("wrong root")
		return false
	}

	var lasthash []byte
	k := index
	for i, hash := range path {
		if i == 0 {
			h := sha256.New()
			h.Write(hash)
			lasthash = h.Sum(nil)
		} else if i == pathlen-1 {
			return bytes.Equal(lasthash, hash)
		} else {
			if k%2 == 0 {
				h := sha256.New()
				h.Write(lasthash)
				h.Write(hash)
				lasthash = h.Sum(nil)
			} else {
				h := sha256.New()
				h.Write(hash)
				h.Write(lasthash)
				lasthash = h.Sum(nil)
			}
			k = k / 2
		}
	}
	return false
}
