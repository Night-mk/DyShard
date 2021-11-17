/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/16/21$ 5:21 AM$
 **/
package load

import (
	"bytes"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/harmony-one/harmony/core/state"
	"testing"
)

// Tests that updating a state trie does not leak any database writes prior to
// actually committing the state.
// 如果没有在statedb层commit，直接调用db的commit方法，不能将数据写入到levelDB
func TestUpdateLeaks(t *testing.T) {
	// Create an empty state database
	// 测试使用memoryDatabase()
	db := rawdb.NewMemoryDatabase()
	state, _ := New(common.Hash{}, state.NewDatabase(db))

	// Update it with some accounts
	for i := byte(0); i < 20; i++ {
		addr := common.BytesToAddress([]byte{i})
		state.SetTargetShardID(addr, uint32(i))
		t.Log(state.loadMapObjects[addr].data)
	}

	root := state.IntermediateRoot(false)
	t.Log("root_hash: ", root)
	// 这里测试调用的是trie/database 的commit，这个commit会使用batch.write
	// 区分loadmapdb的Commit函数，loadmapdb.Commit(false)只提交数据到
	if err := state.Database().TrieDB().Commit(root, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", root.Hex())
	}
	// Ensure that no data was leaked into the database
	//it = transDb.NewIterator(nil, nil)
	it := db.NewIterator()
	t.Log("test0: ")
	for it.Next() {
		t.Log("test: ", it.Key())
		t.Errorf("State leaked into database: %x -> %x", it.Key(), it.Value())
	}
	it.Release()
}

// TestIntermediateLeaks 使用memorydb做测试就行，真正运行的时候再用levelDB比较方便
// 测试statedb存储数据到db.dirties缓存 + 真正提交到disk层的levelDB
// 测试没有中间状态的对象是存在数据库里的, 测试两个相同的状态写到数据库之后，Get得到的结果是否相同
func TestIntermediateLeaks(t *testing.T) {
	// Create two state databases, one transitioning to the final state, the other final from the beginning
	transDb := rawdb.NewMemoryDatabase()
	finalDb := rawdb.NewMemoryDatabase()
	transState, _ := New(common.Hash{}, state.NewDatabase(transDb))
	finalState, _ := New(common.Hash{}, state.NewDatabase(finalDb))

	//transRoot0, err := transState.Commit(false)
	//t.Log(transRoot0)

	modify := func(state *LoadMapDB, addr common.Address, i, tweak byte) {
		state.SetTargetShardID(addr, uint32(int64(i)+int64(tweak)))
	}

	// Modify the transient state.
	for i := byte(0); i < 20; i++ {
		modify(transState, common.Address{i}, i, 0)
	}
	// Write modifications to trie.
	transState.IntermediateRoot(false)

	// Overwrite all the data with new values in the transient database.
	for i := byte(0); i < 20; i++ {
		modify(transState, common.Address{i}, i, 99)
		modify(finalState, common.Address{i}, i, 99)
		fmt.Println(common.Address{i},"shardID", transState.GetTargetShardID(common.Address{i}))
	}

	// Commit and cross check the databases.
	// loadmapdb.commit -> db.insert 将所有节点加入cacheNode，改变 db.dirties
	transRoot, err := transState.Commit(false)
	if err != nil {
		t.Fatalf("failed to commit transition state: %v", err)
	}
	// trie/db.commit -> db.commit batch.write提交到levelDB
	if err = transState.Database().TrieDB().Commit(transRoot, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", transRoot.Hex())
	}

	t.Log("final state commit")

	finalRoot, err := finalState.Commit(false)
	if err != nil {
		t.Fatalf("failed to commit final state: %v", err)
	}
	if err = finalState.Database().TrieDB().Commit(finalRoot, false); err != nil {
		t.Errorf("can not commit trie %v to persistent database", finalRoot.Hex())
	}
	t.Log("final root: ",finalRoot)

	keyData := common.Address{2}
	lmobj := transState.getLoadMapObject(keyData)
	t.Log("getLoadMapObject", lmobj)
	fmt.Println("")

	// 从memorydb读取数据查看结果 利用iterator
	//it = transDb.NewIterator(nil, nil)
	it := finalDb.NewIterator()
	for it.Next() {
		key, fvalue := it.Key(), it.Value()
		tvalue, err := transDb.Get(key)
		fmt.Println("finaldb key: ", key)
		fmt.Println("value: ", tvalue)

		//keyData := common.Address{2}
		//fmt.Println("key data: ", keyData.Bytes())
		//// try to resolve from database
		//enc, err := transState.trie.TryGet(keyData.Bytes())
		//if err != nil {
		//	t.Log("trie.TryGet error: ", err)
		//}
		//fmt.Println("value from tryget: ", enc)
		//data := new(MapAccount)
		//// 将RLP数据从byte解析为value
		//if err := rlp.DecodeBytes(enc, data); err != nil {
		//	t.Log("Failed to decode load map object", "addr", key, "err", err)
		//}
		//t.Log("decode value: ", reflect.TypeOf(data).String())
		fmt.Println("")

		//data := new(MapAccount)
		//if err := rlp.DecodeBytes(tvalue, data); err != nil {
		//	log.Error("Failed to decode load map object", "addr", key, "err", err)
		//}
		//t.Log("decode value: ", reflect.TypeOf(data).String())
		//t.Log("decode value: ", *data)

		if err != nil {
			t.Errorf("entry missing from the transition database: %x -> %x", key, fvalue)
		}
		if !bytes.Equal(fvalue, tvalue) {
			t.Errorf("the value associate key %x is mismatch,: %x in transition database ,%x in final database", key, tvalue, fvalue)
		}
	}
	it.Release()

	//it = transDb.NewIterator(nil, nil)
	it = transDb.NewIterator()
	for it.Next() {
		key, tvalue := it.Key(), it.Value()
		fvalue, err := finalDb.Get(key)
		if err != nil {
			t.Errorf("extra entry in the transition database: %x -> %x", key, it.Value())
		}
		if !bytes.Equal(fvalue, tvalue) {
			t.Errorf("the value associate key %x is mismatch,: %x in transition database ,%x in final database", key, tvalue, fvalue)
		}
	}
}