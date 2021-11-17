/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/16/21$ 12:17 AM$
 **/
package load

import (
	"github.com/ethereum/go-ethereum/common"
)

// journalEntry is a modification entry in the state change journal that can be
// reverted on demand.
type journalEntry interface {
	// revert undoes the changes introduced by this journal entry.
	revert(*LoadMapDB)
	// dirtied returns the Ethereum address modified by this journal entry.
	dirtied() *common.Address
}

// journal contains the list of state modifications applied since the last state
// commit. These are tracked to be able to be reverted in case of an execution
// exception or revertal request.
type journal struct {
	entries []journalEntry         // Current changes tracked by the journal
	dirties map[common.Address]int // Dirty accounts and the number of changes
}

// newJournal create a new initialized journal.
func newJournal() *journal {
	return &journal{
		dirties: make(map[common.Address]int),
	}
}

// append inserts a new modification entry to the end of the change journal.
func (j *journal) append(entry journalEntry) {
	j.entries = append(j.entries, entry)
	if addr := entry.dirtied(); addr != nil {
		j.dirties[*addr]++
	}
}

// revert undoes a batch of journalled modifications along with any reverted
// dirty handling too.
func (j *journal) revert(loadmapdb *LoadMapDB, snapshot int) {
	for i := len(j.entries) - 1; i >= snapshot; i-- {
		// Undo the changes made by the operation
		j.entries[i].revert(loadmapdb)

		// Drop any dirty tracking induced by the change
		if addr := j.entries[i].dirtied(); addr != nil {
			if j.dirties[*addr]--; j.dirties[*addr] == 0 {
				delete(j.dirties, *addr)
			}
		}
	}
	j.entries = j.entries[:snapshot]
}

// dirty explicitly sets an address to dirty, even if the change entries would
// otherwise suggest it as clean. This method is an ugly hack to handle the RIPEMD
// precompile consensus exception.
func (j *journal) dirty(addr common.Address) {
	j.dirties[addr]++
}

// length returns the current number of entries in the journal.
func (j *journal) length() int {
	return len(j.entries)
}

type (
	// Changes to the account trie.
	createObjectChange struct {
		account *common.Address
	}
	resetObjectChange struct {
		prev         *loadMapObject
		//prevdestruct bool
	}
	suicideChange struct {
		account     *common.Address
		prev        bool // whether account had already suicided
		previd uint32
	}

	// Changes to individual accounts.
	targetShardIDChange struct {
		account *common.Address
		prev uint32
	}
	// Changes to other state values.
	addLogChange struct {
		txhash common.Hash
	}
	//balanceChange struct {
	//	account *common.Address
	//	prev    *big.Int
	//}
	//nonceChange struct {
	//	account *common.Address
	//	prev    uint64
	//}
	// Quorum - changes to AccountExtraData
	//accountExtraDataChange struct {
	//	account *common.Address
	//	prev    *AccountExtraData
	//}
)

func (ch createObjectChange) revert(s *LoadMapDB) {
	delete(s.loadMapObjects, *ch.account)
	delete(s.loadMapObjectsDirty, *ch.account)
}

func (ch createObjectChange) dirtied() *common.Address {
	return ch.account
}

func (ch resetObjectChange) revert(l *LoadMapDB) {
	l.setLoadMapObject(ch.prev)
	//if !ch.prevdestruct && l.snap != nil {
	//	delete(l.snapDestructs, ch.prev.addrHash)
	//}
}

func (ch resetObjectChange) dirtied() *common.Address {
	return nil
}

func (ch suicideChange) revert(l *LoadMapDB) {
	obj := l.getLoadMapObject(*ch.account)
	if obj != nil {
		obj.suicided = ch.prev
		obj.setTargetShardID(ch.previd)
		//obj.setBalance(ch.prevbalance)
	}
}

func (ch suicideChange) dirtied() *common.Address {
	return ch.account
}

var ripemd = common.HexToAddress("0000000000000000000000000000000000000003")


//func (ch balanceChange) revert(s *LoadMapDB) {
//	s.getStateObject(*ch.account).setBalance(ch.prev)
//}
//
//func (ch balanceChange) dirtied() *common.Address {
//	return ch.account
//}


// 动态分片
func (ch targetShardIDChange) revert(l *LoadMapDB) {
	l.getLoadMapObject(*ch.account).setTargetShardID(ch.prev)
}

func (ch targetShardIDChange) dirtied() *common.Address {
	return ch.account
}

func (ch addLogChange) revert(l *LoadMapDB) {
	logs := l.logs[ch.txhash]
	if len(logs) == 1 {
		delete(l.logs, ch.txhash)
	} else {
		l.logs[ch.txhash] = logs[:len(logs)-1]
	}
	l.logSize--
}

func (ch addLogChange) dirtied() *common.Address {
	return nil
}