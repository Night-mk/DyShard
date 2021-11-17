/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/16/21$ 12:18 AM$
 **/
package load

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/shard"
	"sort"
)

type revision struct {
	id           int
	journalIndex int
}

// merkleproof of Account
type proofList [][]byte
func (n *proofList) Put(key []byte, value []byte) error {
	*n = append(*n, value)
	return nil
}
func (n *proofList) Delete(key []byte) error {
	panic("not supported")
}


// LoadMapDB需要实现stateDB接口 (core/vm/interface.go)
// LoadMapDB 缓存load-aware mapping到硬盘数据库levelDB的映射
type LoadMapDB struct {
	db   state.Database
	trie state.Trie // 当前所有账户组成的MPT树
	// 删除snapshot
	//snaps         *snapshot.Tree
	//snap          snapshot.Snapshot
	//snapDestructs map[common.Hash]struct{}
	//snapAccounts  map[common.Hash][]byte
	//snapStorage   map[common.Hash]map[common.Hash][]byte

	// 这算是第一级缓存
	// This map holds 'live' objects, which will get modified while processing a state transition.
	loadMapObjects        map[common.Address]*loadMapObject // 存储缓存的账户状态信息
	loadMapObjectsPending map[common.Address]struct{}       // State objects finalized but not yet written to the trie 状态对象已经完成但是还没有写入到Trie中
	loadMapObjectsDirty   map[common.Address]struct{}       // State objects modified in the current execution 在当前执行中修改的状态对象 ，用于后续commit

	// DB error. 记录数据库层面的错误
	// State objects are used by the consensus core and VM which are
	// unable to deal with database-level errors. Any error that occurs
	// during a database read is memoized here and will eventually be returned
	// by StateDB.Commit.
	dbErr error

	// The refund counter, also used by state transitioning.
	refund uint64

	// 存相关tx_hash, block_hash, tx_index, rangemap_hash
	thash, bhash, rhash common.Hash
	txIndex      int
	// 相关区块、交易的日志记录
	logs         map[common.Hash][]*types.Log
	logSize      uint

	// journal记录了状态变化的日志，如果一个状态转移需要n步，日志就记录每一步
	// journal.go 包含每种日志类型操作需要记录的数据，例如：targetShardIDChange中记录了addr, prev_id
	// journal对象包括： entries []journalEntry接口, 每种日志类型都都会实现两个方法revert()dirtied()； dirties addr->int, 记录每个地址状态被修改的次数
	journal        *journal   // 复制了一个和state一样的journal内部变量到mapping包里
	validRevisions []revision // 快照id和journal的长度组成revision，可以回滚
	nextRevisionId int        // 下一个可用的快照id
}

// create a new mapping from a give trie
func New(root common.Hash, db state.Database) (*LoadMapDB, error) {
	tr, err := db.OpenTrie(root)
	if err != nil {
		return nil, err
	}

	ldb := &LoadMapDB{
		db:    db,
		trie:  tr,
		//snaps: snaps,
		loadMapObjects:         make(map[common.Address]*loadMapObject),
		loadMapObjectsPending:  make(map[common.Address]struct{}),
		loadMapObjectsDirty:    make(map[common.Address]struct{}),
		logs:                 make(map[common.Hash][]*types.Log),
		//preimages:            make(map[common.Hash][]byte),
		journal: newJournal(),
	}

	//if ldb.snaps != nil {
	//	if ldb.snap = ldb.snaps.Snapshot(root); ldb.snap != nil {
	//		ldb.snapDestructs = make(map[common.Hash]struct{})
	//		ldb.snapAccounts = make(map[common.Hash][]byte)
	//		ldb.snapStorage = make(map[common.Hash]map[common.Hash][]byte)
	//	}
	//}
	return ldb, nil
}

// create mapping object
func (l *LoadMapDB) createObject(addr common.Address) (newobj, prev *loadMapObject){
	// 可能可以从已删除的状态里恢复数据
	prev = l.getDeletedLoadMapObject(addr)

	// 使用snap记录以前addr的
	//var prevdestruct bool
	//if l.snap != nil && prev != nil {
	//	_, prevdestruct = l.snapDestructs[prev.addrHash]
	//	if !prevdestruct {
	//		l.snapDestructs[prev.addrHash] = struct{}{}
	//	}
	//}

	newobj = newObject(l, addr, MapAccount{})
	//newobj.setNonce(0) // sets the object to dirty
	newobj.setTargetShardID(shard.IniShardID)
	// 使用journal记录历史存储记录
	if prev == nil {
		l.journal.append(createObjectChange{account: &addr})
	} else {
		l.journal.append(resetObjectChange{prev: prev})
	}

	// 设置对象的缓存映射
	l.setLoadMapObject(newobj)

	return newobj, prev
}


// create account object
// 账户使用地址来标记，所以创建账户的时候要传入地址。
// 如果当前的地址已经代表了一个账户，再执行创建账户，会创建1个新的空账户，然后把旧账户的shardID，设置到新的账户
// ???先写着吧，没明白为啥要有这个
func (l *LoadMapDB) CreateMapAccount(addr common.Address){
	newObj, prev := l.createObject(addr)
	if prev != nil {
		newObj.setTargetShardID(prev.data.TargetShardID)
	}
}

// deep copy 深拷贝函数，用于拷贝整个状态db
func (l *LoadMapDB) Copy() *LoadMapDB{
	copyState := &LoadMapDB{
		db: l.db,
		trie: l.db.CopyTrie(l.trie),
		loadMapObjects: make(map[common.Address]*loadMapObject, len(l.journal.dirties)),
		loadMapObjectsPending: make(map[common.Address]struct{}, len(l.loadMapObjectsPending)),
		loadMapObjectsDirty: make(map[common.Address]struct{}, len(l.journal.dirties)),
		refund: l.refund,
		logs: make(map[common.Hash][]*types.Log, len(l.logs)),
		logSize: l.logSize,
		journal: newJournal(),
	}

	// 拷贝 dirty state, logs, preimage(preimage这个到底是干啥的？)
	// journal用来记录每一步的改变
	// 若当前已经存在一个最新区块，该区块之后修改的账户数据首先标记在journal.dirties中，并在提交函数stateDB.Commit中将修改的addr加入到stateObjectPending和stateObjectDirty中。
	for addr := range l.journal.dirties{
		// 如果dirty中有改变的状态对象，则复制该状态缓存到copyState中
		// 不是真的拷贝journal
		if object, exist := l.loadMapObjects[addr]; exist{
			copyState.loadMapObjects[addr] = object.deepCopy(copyState)
			copyState.loadMapObjectsPending[addr] = struct{}{}
			copyState.loadMapObjectsDirty[addr] = struct{}{}
		}
	}
	// 上面的迭代可能是空，所以要对每个缓存都做一次copy尝试
	// copyState里，本状态中pending和dirty中的数据都写入到loadMapObjects中，并且copyState中pending和dirty都是初始化的状态
	for addr := range l.loadMapObjectsPending{
		if _, exist := copyState.loadMapObjects[addr]; !exist{
			// loadMapObjects中不存在该addr的数据时才复制
			copyState.loadMapObjects[addr] = l.loadMapObjects[addr].deepCopy(copyState)
			copyState.loadMapObjectsPending[addr] = struct{}{}
		}
	}
	for addr := range l.loadMapObjectsDirty{
		if _, exist := copyState.loadMapObjects[addr]; !exist{
			copyState.loadMapObjects[addr] = l.loadMapObjects[addr].deepCopy(copyState)
		}
		copyState.loadMapObjectsDirty[addr] = struct{}{}
	}

	for hash, logs := range l.logs {
		cpy := make([]*types.Log, len(logs))
		for i, l1 := range logs {
			cpy[i] = new(types.Log)
			*cpy[i] = *l1
		}
		copyState.logs[hash] = cpy
	}

	return copyState
}


/**
GET mapping object
*/
// getLoadMapObject retrieves a state object given by the address, returning nil if
// the object is not found or was deleted in this execution context. If you need
// to differentiate between non-existent/just-deleted, use getDeletedStateObject.
func (l *LoadMapDB) getLoadMapObject(addr common.Address) *loadMapObject {
	if obj := l.getDeletedLoadMapObject(addr); obj != nil && !obj.deleted {
		return obj
	}
	return nil
}

// getDeletedLoadMapObject is similar to getLoadMapObject, but instead of returning
// nil for a deleted state object, it returns the actual object with the deleted
// flag set. This is needed by the state journal to revert to the correct s-
// destructed object instead of wiping all knowledge about the state object.
func (l *LoadMapDB) getDeletedLoadMapObject(addr common.Address) *loadMapObject {
	// Prefer live objects if any is available
	// 内存中有该对象
	if obj := l.loadMapObjects[addr]; obj != nil {
		//fmt.Println("is from loadMapObjects")
		return obj
	}
	// If no live objects are available, attempt to use snapshots
	// 没有对象，使用snapshot
	var (
		data *MapAccount
		err  error
	)
	// 删除snapshot
	//if l.snap != nil {
	//	// 计算时间的测试
	//	//if metrics.EnabledExpensive {
	//	//	defer func(start time.Time) { l.SnapshotAccountReads += time.Since(start) }(time.Now())
	//	//}
	//	// 在snapshot里添加mapaccount类型
	//	var acc *snap.MapAccount
	//	if acc, err = l.snap.MapAccount(crypto.Keccak256Hash(addr.Bytes())); err == nil {
	//		if acc == nil {
	//			return nil
	//		}
	//		data = &MapAccount{
	//			TargetShardID: acc.TargetShardID,
	//		}
	//	}
	//}

	// If snapshot unavailable or reading from it failed, load from the database
	// 直接从数据库加载
	//if metrics.EnabledExpensive {
	//	defer func(start time.Time) { s.AccountReads += time.Since(start) }(time.Now())
	//}
	// 使用Tire的方法，TryGet
	enc, err := l.trie.TryGet(addr.Bytes())
	if err != nil {
		l.setError(fmt.Errorf("getDeletedLoadMapObject (%x) error: %v", addr.Bytes(), err))
		return nil
	}
	if len(enc) == 0 {
		return nil
	}
	data = new(MapAccount)
	// 将RLP数据从byte解析为value, 这里是为了干啥？
	if err := rlp.DecodeBytes(enc, data); err != nil {
		log.Error("Failed to decode load map object", "addr", addr, "err", err)
		return nil
	}
	//if l.snap == nil || err != nil {
	//}
	// Insert into the live set
	obj := newObject(l, addr, *data)
	l.setLoadMapObject(obj)
	return obj
}

// Retrieve a load map object or create a new load map object if nil.
func (l *LoadMapDB) GetOrNewLoadMapObject(addr common.Address) *loadMapObject {
	loadMapObject := l.getLoadMapObject(addr)
	if loadMapObject == nil {
		loadMapObject, _= l.createObject(addr)
	}
	return loadMapObject
}

/**
UPDATE mapping object
*/

// updateLoadMapObject writes the given object to the trie.
func (l *LoadMapDB) updateLoadMapObject(obj *loadMapObject) {
	// Track the amount of time wasted on updating the account from the trie
	//if metrics.EnabledExpensive {
	//	defer func(start time.Time) { s.AccountUpdates += time.Since(start) }(time.Now())
	//}
	// Encode the account and update the account trie
	addr := obj.Address()

	data, err := rlp.EncodeToBytes(obj)
	if err != nil {
		panic(fmt.Errorf("can't encode object at %x: %v", addr[:], err))
	}
	if err = l.trie.TryUpdate(addr[:], data); err != nil {
		l.setError(fmt.Errorf("updateStateObject (%x) error: %v", addr[:], err))
	}
	// If state snapshotting is active, cache the data til commit. Note, this
	// update mechanism is not symmetric to the deletion, because whereas it is
	// enough to track account updates at commit time, deletions need tracking
	// at transaction boundary level to ensure we capture state clearing.

	// 删除snapshot
	//if l.snap != nil {
	//	l.snapAccounts[obj.addrHash] = snapshot.MapAccountRLP(obj.data.TargetShardID)
	//}

}

// 设置一级缓存LoadMapObject key->value
func (l *LoadMapDB) setLoadMapObject(object *loadMapObject) {
	l.loadMapObjects[object.Address()] = object
}

/**
DELETE mapping object
*/
// deleteLoadMapObject removes the given object from the state trie.
func (l *LoadMapDB) deleteLoadMapObject(obj *loadMapObject) {
	// Track the amount of time wasted on deleting the account from the trie
	//if metrics.EnabledExpensive {
	//	defer func(start time.Time) { s.AccountUpdates += time.Since(start) }(time.Now())
	//}
	// Delete the account from the trie
	addr := obj.Address()
	if err := l.trie.TryDelete(addr[:]); err != nil {
		l.setError(fmt.Errorf("deleteStateObject (%x) error: %v", addr[:], err))
		return
	}
	//s.setError(s.accountExtraDataTrie.TryDelete(addr[:]))
}

// snapshot 返回当前修订的标识符，在validRevision数组里添加当前标识符
// Snapshot returns an identifier for the current revision of the state.
func (l *LoadMapDB) Snapshot() int {
	id := l.nextRevisionId
	l.nextRevisionId++
	l.validRevisions = append(l.validRevisions, revision{id, l.journal.length()})
	return id
}

// revert snapshot
// RevertToSnapshot reverts all state changes made since the given revision.
func (l *LoadMapDB) RevertToSnapshot(revid int) {
	// Find the snapshot in the stack of valid snapshots.
	idx := sort.Search(len(l.validRevisions), func(i int) bool {
		return l.validRevisions[i].id >= revid
	})
	if idx == len(l.validRevisions) || l.validRevisions[idx].id != revid {
		panic(fmt.Errorf("revision id %v cannot be reverted", revid))
	}
	snapshot := l.validRevisions[idx].journalIndex

	// Replay the journal to undo changes and remove invalidated snapshots
	l.journal.revert(l, snapshot)
	l.validRevisions = l.validRevisions[:idx]
}


/**
Finalise和Commit
Finalise代表修改过的状态已经进入“终态”，不会将update
Commit代表所有的状态都写入到数据库
*/
// Finalise 通过删除destructed objects，清空journal，来终结当前状态
// Finalise finalises the state by removing the s destructed objects and clears
// the journal as well as the refunds. Finalise, however, will not push any updates
// into the tries just yet. Only IntermediateRoot or Commit will do that.
func (l *LoadMapDB) Finalise(deleteEmptyObjects bool) {
	for addr := range l.journal.dirties {
		obj, exist := l.loadMapObjects[addr]
		if !exist {
			// ripeMD is 'touched' at block 1714175, in tx 0x1237f737031e40bcde4a8b7e717b2d15e3ecadfe49bb1bbc71ee9deb09c6fcf2
			// That tx goes out of gas, and although the notion of 'touched' does not exist there, the
			// touch-event will still be recorded in the journal. Since ripeMD is a special snowflake,
			// it will persist in the journal even though the journal is reverted. In this special circumstance,
			// it may exist in `s.journal.dirties` but not in `s.stateObjects`.
			// Thus, we can safely ignore it here
			continue
		}
		if obj.suicided || (deleteEmptyObjects && obj.empty()) {
			obj.deleted = true

			// If state snapshotting is active, also mark the destruction there.
			// Note, we can't do this only at the end of a block because multiple
			// transactions within the same block might self destruct and then
			// ressurrect an account; but the snapshotter needs both events.
			// 删除snapshot部分
			//if l.snap != nil {
			//	l.snapDestructs[obj.addrHash] = struct{}{} // We need to maintain account deletions explicitly (will remain set indefinitely)
			//	delete(l.snapAccounts, obj.addrHash)       // Clear out any previously updated account data (may be recreated via a ressurrect)
			//	delete(l.snapStorage, obj.addrHash)        // Clear out any previously updated storage data (may be recreated via a ressurrect)
			//}
		}
		// 加入到loadMapObjectsPending 和 loadMapObjectsDirty
		l.loadMapObjectsPending[addr] = struct{}{}
		l.loadMapObjectsDirty[addr] = struct{}{}
	}
	// Invalidate journal because reverting across transactions is not allowed.
	// 清空journal，没法再回滚了
	l.clearJournalAndRefund()
}

func (l *LoadMapDB) clearJournalAndRefund() {
	if len(l.journal.entries) > 0 {
		l.journal = newJournal()
		l.refund = 0
	}
	l.validRevisions = l.validRevisions[:0] // Snapshots can be created without journal entires
}


// IntermediateRoot 函数计算当前trie的root
// IntermediateRoot computes the current root hash of the state trie.
// It is called in between transactions to get the root hash that
// goes into transaction receipts.
func (l *LoadMapDB) IntermediateRoot(deleteEmptyObjects bool) common.Hash {
	// Finalise all the dirty storage states and write them into the tries
	l.Finalise(deleteEmptyObjects)

	for addr := range l.loadMapObjectsPending {
		obj := l.loadMapObjects[addr]
		if obj.deleted {
			l.deleteLoadMapObject(obj)
		} else {
			//obj.updateRoot(l.db)
			// 更新trie中改动的账户状态，并且更新节点的hash值
			l.updateLoadMapObject(obj)
		}
	}
	if len(l.loadMapObjectsPending) > 0 {
		l.loadMapObjectsPending = make(map[common.Address]struct{})
	}
	// Track the amount of time wasted on hashing the account trie
	//if metrics.EnabledExpensive {
	//	defer func(start time.Time) { l.AccountHashes += time.Since(start) }(time.Now())
	//}
	return l.trie.Hash()
}


// Commit
// Commit writes the state to the underlying in-memory trie database.
// Quorum:
// - linking state root and the AccountExtraData root
func (l *LoadMapDB) Commit(deleteEmptyObjects bool) (common.Hash, error) {
	if l.dbErr != nil {
		return common.Hash{}, fmt.Errorf("commit aborted due to earlier error: %v", l.dbErr)
	}
	// Finalize any pending changes and merge everything into the tries
	// 遍历脏账户，将被更新的账户写入状态树
	l.IntermediateRoot(deleteEmptyObjects)

	// Commit objects to the trie, measuring the elapsed time
	// 将stateObject commit到trie数据库
	if len(l.loadMapObjectsDirty) > 0 {
		l.loadMapObjectsDirty = make(map[common.Address]struct{})
	}
	// Write the account trie changes, measuing the amount of wasted time
	//var start time.Time
	//if metrics.EnabledExpensive {
	//	start = time.Now()
	//}
	var account MapAccount
	// 将状态树写入数据库
	// 报错啥意思？？
	root, err := l.trie.Commit(func(leaf []byte, parent common.Hash) error {
		if err := rlp.DecodeBytes(leaf, &account); err != nil {
			return nil
		}
		//if account.Root != emptyRoot {
		//	s.db.TrieDB().Reference(account.Root, parent)
		//}
		//code := common.BytesToHash(account.CodeHash)
		//if code != emptyCode {
		//	s.db.TrieDB().Reference(code, parent)
		//}
		return nil
	})
	//if metrics.EnabledExpensive {
	//	s.AccountCommits += time.Since(start)
	//}
	// If snapshotting is enabled, update the snapshot tree with this new version
	// 删除snapshot部分
	//if l.snap != nil {
	//	//if metrics.EnabledExpensive {
	//	//	defer func(start time.Time) { l.SnapshotCommits += time.Since(start) }(time.Now())
	//	//}
	//	// Only update if there's a state transition (skip empty Clique blocks)
	//	if parent := l.snap.Root(); parent != root {
	//		if err := l.snaps.Update(root, parent, l.snapDestructs, l.snapAccounts, l.snapStorage); err != nil {
	//			log.Warn("Failed to update snapshot tree", "from", parent, "to", root, "err", err)
	//		}
	//		if err := l.snaps.Cap(root, 127); err != nil { // Persistent layer is 128th, the last available trie
	//			log.Warn("Failed to cap snapshot tree", "root", root, "layers", 127, "err", err)
	//		}
	//	}
	//	l.snap, l.snapDestructs, l.snapAccounts, l.snapStorage = nil, nil, nil, nil
	//}

	return root, err
}



/**
工具方法
*/
// Error
// setError remembers the first non-nil error it is called with.
func (l *LoadMapDB) setError(err error) {
	if l.dbErr == nil {
		l.dbErr = err
	}
}

// Reset 清除所有暂时的状态对象，但是保留trie里的状态数据
// Reset clears out all ephemeral state objects from the state db, but keeps
// the underlying state trie to avoid reloading data for the next operations.
func (l *LoadMapDB) Reset(root common.Hash) error {
	tr, err := l.db.OpenTrie(root)
	if err != nil {
		return err
	}
	l.trie = tr
	l.loadMapObjects = make(map[common.Address]*loadMapObject)
	l.loadMapObjectsPending = make(map[common.Address]struct{})
	l.loadMapObjectsDirty = make(map[common.Address]struct{})
	l.thash = common.Hash{}
	l.bhash = common.Hash{}
	l.txIndex = 0
	l.logs = make(map[common.Hash][]*types.Log)
	l.logSize = 0
	//s.preimages = make(map[common.Hash][]byte)
	l.clearJournalAndRefund()

	//if l.snaps != nil {
	//	l.snapAccounts, l.snapDestructs, l.snapStorage = nil, nil, nil
	//	if l.snap = l.snaps.Snapshot(root); l.snap != nil {
	//		l.snapDestructs = make(map[common.Hash]struct{})
	//		l.snapAccounts = make(map[common.Hash][]byte)
	//		l.snapStorage = make(map[common.Hash]map[common.Hash][]byte)
	//	}
	//}
	return nil
}

// Log
// add log
func (l *LoadMapDB) AddLog(log *types.Log) {
	l.journal.append(addLogChange{txhash: l.thash})

	log.TxHash = l.thash
	log.BlockHash = l.bhash
	log.TxIndex = uint(l.txIndex)
	log.Index = l.logSize
	l.logs[l.thash] = append(l.logs[l.thash], log)
	l.logSize++
}
// get log
func (l *LoadMapDB) GetLogs(hash common.Hash) []*types.Log {
	return l.logs[hash]
}
// logs
func (l *LoadMapDB) Logs() []*types.Log {
	var logs []*types.Log
	for _, lgs := range l.logs {
		logs = append(logs, lgs...)
	}
	return logs
}

// Exist reports whether the given account address exists in the state.
// Notably this also returns true for suicided accounts.
func (l *LoadMapDB) Exist(addr common.Address) bool {
	return l.getLoadMapObject(addr) != nil
}

// Empty returns whether the state object is either non-existent
// or empty according to the EIP161 specification (balance = nonce = code = 0)
func (l *LoadMapDB) Empty(addr common.Address) bool {
	so := l.getLoadMapObject(addr)
	return so == nil || so.empty()
}


// get target shard id
// Retrieve the balance from the given address or shardcommon.If object not found 返回iniShardID
func (l *LoadMapDB) GetTargetShardID(addr common.Address) uint32 {
	loadMapObject := l.getLoadMapObject(addr)
	if loadMapObject != nil {
		return loadMapObject.TargetShardID()
	}
	return shard.IniShardID
}

// Block, TxIndex
// TxIndex returns the current transaction index set by Prepare.
func (l *LoadMapDB) TxIndex() int {
	return l.txIndex
}

func (l *LoadMapDB) TxHash() common.Hash {
	return l.thash
}

// BlockHash returns the current block hash set by Prepare.
func (l *LoadMapDB) BlockHash() common.Hash {
	return l.bhash
}

// GetProof returns the MerkleProof for a given Account
func (l *LoadMapDB) GetProof(a common.Address) ([][]byte, error) {
	var proof proofList
	err := l.trie.Prove(crypto.Keccak256(a.Bytes()), 0, &proof)
	return [][]byte(proof), err
}

// Database retrieves the low level database supporting the lower level trie ops.
func (l *LoadMapDB) Database() state.Database {
	return l.db
}

/**
Setting
*/
// 设置thash, bhash, txIndex(说实话不知道有啥用)



// 设置shardID
func (l *LoadMapDB) SetTargetShardID(addr common.Address, id uint32) {
	loadMapObject := l.GetOrNewLoadMapObject(addr)
	if loadMapObject != nil {
		loadMapObject.SetTargetShardID(id)
	}
}


// ?? 没搞懂suicide 到底是个什么状态
// Suicide marks the given account as suicided.
// This clears the account balance.
//
// The account's state object is still available until the state is committed,
// getStateObject will return a non-nil account after Suicide.
func (l *LoadMapDB) Suicide(addr common.Address) bool {
	loadMapObject := l.getLoadMapObject(addr)
	if loadMapObject == nil {
		return false
	}
	l.journal.append(suicideChange{
		account:     &addr,
		prev:        loadMapObject.suicided,
		//prevbalance: new(big.Int).Set(loadMapObject.Balance()),
		previd: loadMapObject.TargetShardID(),

	})
	loadMapObject.markSuicided()
	loadMapObject.data.TargetShardID = shard.IniShardID

	return true
}