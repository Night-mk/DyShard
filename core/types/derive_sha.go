// Copyright 2014 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
)

// DerivableBase ..
type DerivableBase interface {
	Len() int
	GetRlp(i int) []byte
}

// DerivableList is the interface of DerivableList.
type DerivableList interface {
	DerivableBase
	ToShardID(i int) uint32
	MaxToShardID() uint32 // return the maximum non-empty destination shardID
}

// DeriveSha calculates the hash of the trie generated by DerivableList.
// 计算通过DerivableList生成的trie的哈希值，用于transaction，receipt的root哈希值计算
func DeriveSha(list ...DerivableBase) common.Hash {
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	var num uint

	for j := range list {
		for i := 0; i < list[j].Len(); i++ {
			keybuf.Reset()
			rlp.Encode(keybuf, num)
			trie.Update(keybuf.Bytes(), list[j].GetRlp(i))
			num++
		}
	}
	return trie.Hash()
}

//// Legacy forked logic. Keep as is, but do not use it anymore ->

// DeriveOneShardSha calculates the hash of the trie of
// cross shard transactions with the given destination shard
func DeriveOneShardSha(list DerivableList, shardID uint32) common.Hash {
	keybuf := new(bytes.Buffer)
	trie := new(trie.Trie)
	for i := 0; i < list.Len(); i++ {
		if list.ToShardID(i) != shardID {
			continue
		}
		keybuf.Reset()
		rlp.Encode(keybuf, uint(i))
		trie.Update(keybuf.Bytes(), list.GetRlp(i))
	}
	return trie.Hash()
}

// DeriveMultipleShardsSha calcualtes the root hash of tries generated by DerivableList of multiple shards
// If the list is empty, then return EmptyRootHash
// else, return |shard0|trieHash0|shard1|trieHash1|...| for non-empty destination shards
func DeriveMultipleShardsSha(list DerivableList) common.Hash {
	by := []byte{}
	if list.Len() == 0 {
		return EmptyRootHash
	}
	for i := 0; i <= int(list.MaxToShardID()); i++ {
		shardHash := DeriveOneShardSha(list, uint32(i))
		if shardHash == EmptyRootHash {
			continue
		}
		sKey := make([]byte, 4)
		binary.BigEndian.PutUint32(sKey, uint32(i))
		by = append(by, sKey...)
		by = append(by, shardHash[:]...)
	}
	if len(by) == 0 {
		return EmptyRootHash
	}
	return crypto.Keccak256Hash(by)
}

//// <- Legacy forked logic. Keep as is, but do not use it anymore
