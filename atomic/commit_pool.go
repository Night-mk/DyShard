/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/2/21$ 12:42 AM$
 **/
package atomic

import (
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/event"
	"github.com/harmony-one/harmony/atomic/types"
	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard/mapping/load"
	"math"
	"math/big"
	"sync"
	"time"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 100
	// 暂定每个区块最少需要处理的迁移交易数量
	//minProcessSttxNum = 200
	minProcessSttxNum = 1
)
var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

/**
	CommitPool 暂时存储原子提交的事务，事务提交后交给个分片内共识真正修改状态
 */
type CommitPool struct {
	chain        *core.BlockChain
	chainHeadCh  chan core.ChainHeadEvent     // txpool订阅区块头的消息
	chainHeadSub event.Subscription      // 区块头消息订阅器，通过它可以取消消息(除了取消订阅也没啥别的用)
	mu sync.RWMutex // 锁数据结构

	currentState *state.DB
	pendingState  *state.ManagedState // Pending state tracking virtual nonces  假设pending列表最后一个交易执行完后应该的状态
	currentLoadMapState *load.LoadMapDB

	// 第一版实现，加common.Address对实验没有任何作用
	// freezeCache map[common.Address] *types.CacheShard
	// cached map[common.Address] *types.CacheShard
	// committed map[common.Address] *types.CacheShard
	freezeCache *types.CacheShard // freezeCache 缓存propose提交的事务，用于构建stateTransfer交易，在source shard锁定address的状态
	cached *types.CacheShard      // cached 缓存2PC中第一阶段propose提交的事务，以事务内容中的address作为key
	committed *types.CacheShard   // committed 缓存2PC中第二阶段commit提交的事务，提交阶段会将cached中的内容转移到committed中，committed中的数据最终被添加到最新block中执行
	wg sync.WaitGroup                                 // for shutdown sync

	Signal map[uint64] chan int // 添加通知2PC消息的信号量，propose=1, commit=2

	committedTxList map[uint64] int // 存一个已经完成2PC提交的交易的列表，存一下目前某个index的迁移交易是否已经完成
	freezeReadTxList map[uint64] int // 记录tx是否被newblock读取过
	committedReadTxList map[uint64] int
}

// 初始化CommitPool
// 主要功能
// 1. 初始化CommitPool类，从本地文件中加载local交易
// 2. 订阅规范链更新事件
// 3. 启动事件监听
func NewCommitPool(chain *core.BlockChain) *CommitPool{
	pool := &CommitPool{
		chain: chain,
		freezeCache: types.NewCache(),
		cached: types.NewCache(),
		committed: types.NewCache(),
		chainHeadCh: make(chan core.ChainHeadEvent, chainHeadChanSize), // 初始化容量为10的channel
		Signal: make(map[uint64] chan int),
		committedTxList: make(map[uint64] int),
		freezeReadTxList: make(map[uint64] int),
		committedReadTxList: make(map[uint64] int),
	}

	// 重置pool
	pool.reset(nil, chain.CurrentBlock().Header())

	// 订阅规范链更新事件
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return 启动事件监听
	pool.wg.Add(1)
	// 异步启动事件监听
	go pool.loop()

	return pool
}

/*
	GET函数
 */
// 返回freezeCache缓存中的数据的拷贝
func (pool *CommitPool) FreezeCache() ([]types.StateTransferTransaction, error){
	// 使用写锁？
	pool.mu.Lock()
	defer pool.mu.Unlock()
	// 利用Count计算freezeCache的容量
	cache := make([]types.StateTransferTransaction, 0, pool.freezeCache.Count())
	utils.Logger().Info().Msgf("FreezeCache get tx number: %d", pool.freezeCache.Count())
	// 限制在写入一定数量时才读取
	if pool.freezeCache.Count() < minProcessSttxNum{
		return make([]types.StateTransferTransaction, 0, 0), nil
	}
	pool.freezeCache.Range(func(index uint64, tx types.StateTransferTransaction) bool {
		if _, ok := pool.freezeReadTxList[index]; !ok{ // 判断tx是否已经被获取过
			cache = append(cache, tx)
			pool.freezeReadTxList[index] = 1
		}
		return true
	})

	return cache, nil
}

// 读取committed缓存的数据拷贝
func (pool *CommitPool) Committed() ([]types.StateTransferTransaction, error){
	pool.mu.Lock()
	defer pool.mu.Unlock()
	cache := make([]types.StateTransferTransaction, 0, pool.committed.Count())
	// 限制在写入一定数量时才读取
	if pool.committed.Count() < minProcessSttxNum{
		return make([]types.StateTransferTransaction, 0, 0), nil
	}
	pool.committed.Range(func(index uint64, tx types.StateTransferTransaction) bool {
		if _, ok := pool.committedReadTxList[index]; !ok{ // 判断tx是否已经被读取过
			cache = append(cache, tx)
			pool.committedReadTxList[index] = 1
		}
		return true
	})
	//var cache []types.StateTransferTransaction
	//txs := pool.committed.GetStore()
	//if len(txs)>0{
	//	cache = make([]types.StateTransferTransaction, 0, len(txs))
	//	for _, tx := range txs{
	//		cache = append(cache, tx)
	//	}
	//}
	return cache, nil
}

// 读取cached缓存的数据拷贝
func (pool *CommitPool) Cached() ([]types.StateTransferTransaction, error){
	pool.mu.Lock()
	defer pool.mu.Unlock()
	cache := make([]types.StateTransferTransaction, 0, pool.cached.Count())
	pool.cached.Range(func(index uint64, tx types.StateTransferTransaction) bool {
		cache = append(cache, tx)
		return true
	})
	//var cache []types.StateTransferTransaction
	//txs := pool.cached.GetStore()
	//if len(txs)>0{
	//	cache = make([]types.StateTransferTransaction, 0, len(txs))
	//	for _, tx := range txs{
	//		if _, ok := pool.committedReadTxList[tx.Index()]; !ok{ // 判断tx是否已经被读取过
	//			cache = append(cache, tx)
	//			pool.committedReadTxList[tx.Index()] = 1
	//		}
	//	}
	//}

	return cache, nil
}

// 获取cached中的数据
func (pool *CommitPool) CachedById(index uint64) (types.StateTransferTransaction, bool){
	message, ok := pool.cached.Get(index)
	return message, ok
}

// 新增：添加一条新交易atomic msg到freezeCache
func (pool *CommitPool) addTxFreeze(tx *types.StateTransferTransaction){
	pool.freezeCache.Set(tx.Index(), *tx)
}

func (pool *CommitPool) deleteTxFromFreeze(index uint64){
	pool.freezeCache.Delete(index)
}

// 新增：添加一条新交易atomic msg到cached
func (pool *CommitPool) addTxCached(tx *types.StateTransferTransaction){
	pool.cached.Set(tx.Index(), *tx)
}

// 删除：从cached删除一条交易：根据address和交易编号index查找删除
func (pool *CommitPool) deleteTxFromCached(index uint64){
	pool.cached.Delete(index)
}

// 在committed中添加一条交易
func (pool *CommitPool) addTxCommitted(tx *types.StateTransferTransaction){
	pool.committed.Set(tx.Index(), *tx)
}

// 删除：从committed中删除一条交易
// 用途：1. Source分片将Committed中的所有交易写入LoadAwareMap后删除相关交易的事务
// 2. Target分片将Committed中的所有交易写入区块后删除相关交易的事务
func (pool *CommitPool) deleteTxFromCommitted(index uint64){
	pool.committed.Delete(index)
}


// propose阶段，添加一条新的atomic msg到freezeCached
func (pool *CommitPool) addFreezeTransaction(index uint64, source uint32, target uint32, address common.Address) error{
	pool.mu.Lock()
	defer pool.mu.Unlock()
	// 1. 将State添加到commitPool freezeCache
	// 根据消息初始化tx
	newTx := types.NewStateTransferTransaction(
		source, target, 0, address, big.NewInt(0), 0, []byte{}, true, index)
	pool.addTxFreeze(newTx)
	// 2. 在commitPool中记录一个Signal
	//fmt.Println("addTxFreeze END: ", index)
	//_, ok := pool.Signal[index]
	ok := pool.GetSignal(index)
	if !ok{
		pool.InitSignal(index)
	}
	return nil
	//fmt.Println("CommitPool END addFreezeTransaction")
}

// 根据区块中写入的的sttx，在cached中添加一条缓存
// (分离 cached添加交易 和 freezeCached删除交易的逻辑)
func (pool *CommitPool) addCachedTransaction(tx *types.StateTransferTransaction){
	// 这里没有冲突，应该不需要读写锁
	//pool.mu.Lock()
	//defer pool.mu.Unlock()
	// 从cached中获取交易，判断cached中是否已经插入过该index的交易
	// 构建一个新交易缓到cached中
	newTx := tx.Copy()
	// 将该交易的ToFreeze改变为false
	newTx.SetToFreeze(false)
	_, ok := pool.cached.Get(tx.Index())
	if !ok{ // 为cached添加一笔交易
		pool.addTxCached(newTx)
	}else{// 如果已经写入过， 则不做任何处理
		utils.Logger().Info().Msg(fmt.Sprintf("Value has already stored in Cached the index %d, \n address: %s", tx.Index(), tx.Address()))
	}

	//fmt.Println("CommitPool END addCachedTransaction")
}

// 在committed中新增一条交易，根据2PC的commit的消息
func (pool *CommitPool) addCommittedTransaction(source uint32, target uint32,addr common.Address, index uint64, s types.StateData){
	// 原子操作
	pool.mu.Lock()
	defer pool.mu.Unlock()
	// 判断committed列表中是否已经有该交易
	// 利用commit消息构建一个新交易
	newTx := types.NewStateTransferTransaction(source, target, s.StateProof.BlockHeight, addr, s.State.Balance, s.State.Nonce, s.State.Data, false, index)
	_, ok := pool.committed.Get(index)
	if !ok{
		// 为committed添加一笔交易
		pool.addTxCommitted(newTx)
	}else{// 如果已经有该交易了，就不用写入
		utils.Logger().Info().Msg(fmt.Sprintf("No value cache on the index %d, \n address: %s", index, addr))
	}
	//fmt.Println("CommitPool END addCommittedTransaction")
}

func (pool *CommitPool) GetCurrentState() *state.DB{
	return pool.currentState
}

func (pool *CommitPool) GetPendingState() *state.ManagedState{
	return pool.pendingState
}

/*
	SET
 */
// 设置cached缓存的状态（收到commit request的时候调用）
//func (pool *CommitPool) SetCachedState(addr common.Address, index uint64, balance *big.Int, nonce uint64, data []byte){
//	pool.mu.Lock()
//	defer pool.mu.Unlock()
//	pool.cached[addr].SetStateOnly(index, addr, balance, nonce, data)
//}

/*
	Signal 交易处理完成的信号量
	创建channel，通知消息编号=Index的2PC协议执行的协程
 */
// 初始化信号量
func (pool *CommitPool) InitSignal(index uint64){
	//pool.mu.Lock()
	//defer pool.mu.Unlock()
	pool.Signal[index] = make(chan int)
	//fmt.Println("Create signal END with Index=[", index,"]")
}

// 设置信号量，通知2PC的propose，commit协程
// i=1 propose reply success
// i=2 commit reply success
func (pool *CommitPool) SetSignal(index uint64, i int){
	//pool.mu.Lock()
	//defer pool.mu.Unlock()
	pool.Signal[index] <- i
}

func (pool *CommitPool) GetSignal(index uint64) bool{
	//pool.mu.RLock()
	//defer pool.mu.RUnlock()
	_, ok := pool.Signal[index]
	return ok
}

// 结束信号量通道
func (pool *CommitPool) CloseSignal(index uint64){
	pool.mu.Lock()
	defer pool.mu.Unlock()
	close(pool.Signal[index])
}

func (pool *CommitPool) CloseAllSignal(){
	//pool.mu.Lock()
	//defer pool.mu.Unlock()
	for _, sigItem := range pool.Signal{
		close(sigItem)
	}
}

// 记录已经提交的2PC消息的编号
func (pool *CommitPool)RecordCommittedTx(index uint64){
	//pool.mu.Lock()
	//defer pool.mu.Unlock()
	pool.committedTxList[index] = 1
}

func (pool *CommitPool)GetCommittedTx(index uint64) (int, bool){
	msg, ok := pool.committedTxList[index]
	return msg, ok
}

// 重置：接收到规范链更新时，重新整理CommitPool（类似txPool）
// 扫描cached和committed缓存，删除新区块中已经上链的交易对应的Tx
// 扫描freezeCache缓存，将成功的freeze交易
// 函数只在本文件中调用
func (pool *CommitPool) reset(oldHead, newHead *block.Header) {
	// 1. 找到由于规范链更新而作废的交易；
	// 在committed中找到作废交易，删除
	// 在cached中找到相同交易（freezeState），对相关2PC流程发送同步信号

	// 新区快头的父区块不等于老区块，说明新老区块不在同一条链
	// oldHead表示节点存储的当前区块高度的区块历史
	if oldHead != nil && oldHead.Hash() != newHead.ParentHash() {
		oldNum := oldHead.Number().Uint64()
		newNum := newHead.Number().Uint64()

		// 如果新头区块和旧头区块相差大于64，则所有交易不必回退到交易池
		// 处理分叉时的问题
		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			utils.Logger().Debug().Uint64("depth", depth).Msg("Skipping deep transaction reorg")
		} else { // 暂时不处理分叉问题
			//var (
			//	// 最新区块之前某个区块
			//	rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number().Uint64())
			//	// 最新区块头的区块
			//	add = pool.chain.GetBlock(newHead.Hash(), newHead.Number().Uint64())
			//)

			// 如果旧链的区块高度大于最新区块的高度，则旧链向前回退，并回收所有回退的交易到交易池
			// 此时意味着当前链在分叉上，要回退到newHead的高度，所以这中间新区块的交易要discard丢弃
			//for rem.NumberU64() > add.NumberU64() {
			//	for _, tx := range rem.StateTransferTransactions(){
			//	}
			//}
			return
		}

	}

	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}
	// 2. 给交易池设置最新的世界状态
	//statedb, err := pool.chain.StateAt(newHead.Root())
	//if err != nil {
	//	utils.Logger().Error().Err(err).Msg("Failed to reset txpool state")
	//	return
	//}
	/* dynamic sharding */
	// 2. 给交易池设置最新的loadMap状态
	loadMapdb, err := pool.chain.LoadMapStateAt(newHead.LoadMapRoot())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to reset txpool state")
		return
	}
	// 设置新链头区块的状态
	pool.currentLoadMapState = loadMapdb
	statedb, err := pool.chain.StateAt(newHead.Root())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to reset txpool state")
		return
	}
	// 同时缓存一个当前最新区块的state, 读取也是从这个对象读取
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	// 3. 把旧链回退的交易放入交易池(暂时不处理)

	// 4. 从cache(committed)中删除已有交易（不考虑存在无效交易的情况）;只从当前区块的交易里删除committed中index相同的交易
	// 从cache(freezeCache)中转移已经提交的Tx到cached
	// 这里不用管是source还是target，处理交易池都一样
	sttxs := pool.chain.CurrentBlock().StateTransferTransactions()
	//fmt.Println("=====Commit Pool reset current block sttxs=====", sttxs)
	if len(sttxs) > 0{
		for _, sttx := range sttxs{ // 遍历StateTransferTransaction
			index := sttx.Index()
			// 判断是否是freeze交易
			if sttx.ToFreeze(){ // 是freeze操作的交易
				// 由于区块共识，交易写入到cached中时，可能节点还没收到coordinator发送的消息
				// 所以，插入区块时，直接在cached中利用sttx创建cached中的一个缓存【此时也设置freeze为false】
				pool.addCachedTransaction(sttx)
				pool.freezeReadTxList[index] = 1
				// 设置信号量 (只在reset的时候设置信号量) 只有执行2PC的节点会执行设置signal
				if _, ok := pool.freezeCache.Get(index); ok{
					if ok1 := pool.GetSignal(index); !ok1{ // 使用get函数对signal读的时候加锁
						pool.InitSignal(index)
					}
					pool.SetSignal(index, 1)
					// 此时，如果freezeCache也有index相关的交易，就删除(没有也删除)
					pool.deleteTxFromFreeze(index)
				}
				//fmt.Println("=====Commit Pool reset [can set signal]=====")
				//fmt.Println("=====Commit Pool reset [can cache and delete]=====")
			}else{ // 不是freeze操作的交易
				pool.committedReadTxList[index] = 1
				// 通知对应的commit协程, 只有执行2PC的节点设置signal
				if _, ok := pool.committed.Get(index); ok{
					pool.SetSignal(index, 2)
				}
				// 从committed删除交易 (只有真正插入区块的时候，才会从committed删除缓存)
				pool.deleteTxFromCommitted(index)
				// 记录该index交易已经提交
				pool.RecordCommittedTx(index)
			}
		}
	}

}

// 循环监听方法
// 监听区块链的变动，同时开启多个go routine处理不同的事件(仿照TxPool的reset方法写)
func (pool *CommitPool) loop(){
	defer pool.wg.Done()

	// report为每8秒钟报告一次状态  8 * time.Second
	// 交易状态其实就是报告所有cache中的交易数量
	report := time.NewTicker(statsReportInterval)
	defer report.Stop()
	// evict为每分钟检测不活动account的交易移除(暂时不需要)
	// 获取previous区块的索引
	head := pool.chain.CurrentBlock()

	// 循环监听事件信号channel
	for {
		select {
		// Handle ChainHeadEvent 监听规范链更新事件，重置交易池：pool.reset()；
		case ev := <-pool.chainHeadCh:
			//fmt.Println("========CommitPool GET new block=========")
			if ev.Block != nil{
				pool.mu.Lock()
				// 重置交易池[重点]
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block
				pool.mu.Unlock()
			}

		// Be unsubscribed due to system stopped
		// 解除订阅
		case <-pool.chainHeadSub.Err():
			return

		// 定时报告freezeCache，cached，committed的状态
		// 这里就简单统计个数量先(没啥用先不不写了)
		case <-report.C:
			pool.mu.RLock()
			pool.mu.RUnlock()
		}
		// 定时删除超时交易（暂时不考虑）
		// 定时本地储存尚未额打包的local交易（暂时不考虑）
	}

}

