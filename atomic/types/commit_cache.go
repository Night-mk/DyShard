/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/2/21$ 1:03 AM$
 **/
package types

import (
	"sync"
)

/**
	dynamic sharding
	针对分片交易的缓存CacheShard
*/
// 通用原子提交消息
/**
	Source: 发送交易的地址分片
	Target: 接收交易的地址分片
	Value： 交易内容的rlp编码
*/

// 使用stateTransferTransaction格式进行缓存, 缓存不使用指针, 使用交易的拷贝
type CacheShard struct {
	store map[uint64] StateTransferTransaction // key: 事务id
	mu    sync.RWMutex
}

func NewCache() *CacheShard {
	hashtable := make(map[uint64]StateTransferTransaction)
	return &CacheShard{store: hashtable}
}

//func NewCache1(index uint64, transaction StateTransferTransaction) *CacheShard{
//	hashtable := make(map[uint64]StateTransferTransaction)
//	hashtable[index] = transaction
//	return &CacheShard{store: hashtable}
//}


// index标识事务的唯一编号
func (c *CacheShard) Set(index uint64, tx StateTransferTransaction) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[index] = tx
}

// index标识事务的唯一编号
//func (c *CacheShard) SetStateOnly(index uint64, address common.Address, balance *big.Int, nonce uint64, data []byte) {
//	c.mu.Lock()
//	defer c.mu.Unlock()
//	c.store[index].SetState(address, balance, nonce, data)
//}

func (c *CacheShard) Get(index uint64) (StateTransferTransaction, bool){
	c.mu.RLock()
	defer c.mu.RUnlock()
	message, ok := c.store[index]
	return message, ok
}

func (c *CacheShard) Delete(index uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, index)
}

// 读取map里所有的key，value
func (c *CacheShard) Range(f func(index uint64, tx StateTransferTransaction) bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	for key, value := range c.store{
		if !f(key, value){
			break
		}
	}
}

func (c *CacheShard) GetStore() map[uint64] StateTransferTransaction{
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.store
}

func (c *CacheShard) Count() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.store)
}