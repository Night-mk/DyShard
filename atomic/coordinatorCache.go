/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/9/21$ 6:29 AM$
 **/
package atomic

import "sync"

type msg struct {
	Value []byte
}

type Cache struct {
	store map[uint64]msg
	mu    sync.RWMutex
}

func NewCoorCache() *Cache {
	hashtable := make(map[uint64]msg)
	return &Cache{store: hashtable}
}

func (c *Cache) Set(index uint64, value []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store[index] = msg{value}
}

func (c *Cache) Get(index uint64) ([]byte, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	message, ok := c.store[index]
	return message.Value, ok
}

func (c *Cache) Delete(index uint64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.store, index)
}