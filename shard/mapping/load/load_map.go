/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/16/21$ 12:18 AM$
 **/
package load

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/harmony-one/harmony/shard"
)

type Storage map[common.Hash]common.Hash

// loadMapObject 保存load-aware mapping的MPT树对象
// 因为不需要存合约相关的内容，所以结构很简单
type loadMapObject struct {
	address  common.Address
	addrHash common.Hash // hash of ethereum address of the account
	data     MapAccount
	db       *LoadMapDB // 所属的StateDB

	// Cache flags.
	// When an object is marked suicided it will be delete from the trie
	// during the "update" phase of the state transition.
	//dirtyCode bool // true if the code was updated
	suicided  bool
	deleted   bool
}

// MapAccount 标识当前地址被map到哪个目标分片上
type MapAccount struct {
	TargetShardID  uint32
}

// newObject creates a load map object.
func newObject(db *LoadMapDB, address common.Address, data MapAccount) *loadMapObject {
	//if data.TargetShardID == shard.IniShardID {
	//	// 先用0顶替，应该分配当前该节点的shardID
	//	data.TargetShardID = uint32(0)
	//}
	data.TargetShardID = shard.IniShardID
	return &loadMapObject{
		db:             db,
		address:        address,
		addrHash:       crypto.Keccak256Hash(address[:]),
		data:           data,
		//originStorage:  make(Storage),
		//pendingStorage: make(Storage),
		//dirtyStorage:   make(Storage),
	}
}


// setTargetShardID 设置目标分片ID, 用于对状态数据进行设置
func (l *loadMapObject) setTargetShardID(id uint32){
	l.data.TargetShardID = id
}

// SetTargetShardID 用于在journal中记录修改次数 j.dirties[*addr]++
func (l *loadMapObject) SetTargetShardID(id uint32){
	l.db.journal.append(targetShardIDChange{
		account: &l.address,
		prev: l.data.TargetShardID,
	})
	l.setTargetShardID(id)
}

func (l *loadMapObject) TargetShardID() uint32{
	return l.data.TargetShardID
}

// return address of account
func (l *loadMapObject) Address() common.Address {
	return l.address
}

// empty returns whether the account is considered empty.
func (l *loadMapObject) empty() bool {
	return l.data.TargetShardID == shard.IniShardID
}


func (l *loadMapObject) markSuicided() {
	l.suicided = true
}

// 对loadMapObject的深拷贝
func (l *loadMapObject) deepCopy(db *LoadMapDB) *loadMapObject{
	object := newObject(db, l.address, l.data)
	object.suicided = l.suicided
	object.deleted = l.deleted
	return object
}