/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/15/21$ 10:24 PM$
 **/
package _range

import (
	"crypto/sha256"
	"github.com/harmony-one/harmony/shard/shardutil"
	"strconv"
)

/*
	dynamic sharding
*/
// 简化实现range-based mapping
// 实现过程不使用tree结构
type RangeMapContent struct {
	ShardID uint32
	Prefix string
}

// 实现tree_content.content接口
//CalculateHash hashes the values of a TestSHA256Content
// nodeHash = hash_func(path+shardID)
func (r RangeMapContent) CalculateHash() ([]byte, error) {
	h := sha256.New()
	nodeContent := r.Prefix+strconv.FormatUint(uint64(r.ShardID),10)
	if _, err := h.Write([]byte(nodeContent)); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

//Equals tests for equality of two Contents
func (r RangeMapContent) Equals(other shardutil.Content) (bool, error) {
	return r.ShardID == other.GetShardID() && r.Prefix == other.GetPath(), nil
}

func (r RangeMapContent) GetShardID() uint32{
	return r.ShardID
}

func (r RangeMapContent) GetPath() string{
	return r.Prefix
}
