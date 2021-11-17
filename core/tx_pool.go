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

package core

import (
	"fmt"
	"math"
	"math/big"
	"sort"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/prque"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/metrics"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/pkg/errors"

	"github.com/harmony-one/harmony/block"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	hmyCommon "github.com/harmony-one/harmony/internal/common"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	staking "github.com/harmony-one/harmony/staking/types"
)

const (
	// chainHeadChanSize is the size of channel listening to ChainHeadEvent.
	chainHeadChanSize = 10
)

var (
	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrInvalidShard is returned if the transaction is for the wrong shard.
	ErrInvalidShard = errors.New("invalid shard")

	// ErrNonceTooLow is returned if the nonce of a transaction is lower than the
	// one present in the local chain.
	ErrNonceTooLow = errors.New("nonce too low")

	// ErrUnderpriced is returned if a transaction's gas price is below the minimum
	// configured for the transaction pool.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

	// ErrInsufficientFunds is returned if the total cost of executing a transaction
	// is higher than the balance of the user's account.
	ErrInsufficientFunds = errors.New("insufficient funds for gas * price + value")

	// ErrIntrinsicGas is returned if the transaction is specified to use less gas
	// than required to start the invocation.
	ErrIntrinsicGas = errors.New("intrinsic gas too low")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure noone is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrKnownTransaction is returned if a transaction that is already in the pool
	// attempting to be added to the pool.
	ErrKnownTransaction = errors.New("known transaction")

	// ErrInvalidMsgForStakingDirective is returned if a staking message does not
	// match the related directive
	ErrInvalidMsgForStakingDirective = errors.New("staking message does not match directive message")

	// ErrBlacklistFrom is returned if a transaction's from/source address is blacklisted
	ErrBlacklistFrom = errors.New("`from` address of transaction in blacklist")

	// ErrBlacklistTo is returned if a transaction's to/destination address is blacklisted
	ErrBlacklistTo = errors.New("`to` address of transaction in blacklist")

	/* dynamic sharding */
	// ErrAddrHasFreezed 返回，表示交易相关的账户已经锁定，无法执行或加入该交易
	ErrAddrHasFreezed = errors.New("err: 交易相关账户已经锁定")
)

var (
	evictionInterval    = time.Minute     // Time interval to check for evictable transactions
	statsReportInterval = 8 * time.Second // Time interval to report transaction pool stats
)

var (
	// Metrics for the pending pool
	pendingDiscardCounter   = metrics.NewRegisteredCounter("txpool/pending/discard", nil)
	pendingReplaceCounter   = metrics.NewRegisteredCounter("txpool/pending/replace", nil)
	pendingRateLimitCounter = metrics.NewRegisteredCounter("txpool/pending/ratelimit", nil) // Dropped due to rate limiting
	pendingNofundsCounter   = metrics.NewRegisteredCounter("txpool/pending/nofunds", nil)   // Dropped due to out-of-funds

	// Metrics for the queued pool
	queuedDiscardCounter   = metrics.NewRegisteredCounter("txpool/queued/discard", nil)
	queuedReplaceCounter   = metrics.NewRegisteredCounter("txpool/queued/replace", nil)
	queuedRateLimitCounter = metrics.NewRegisteredCounter("txpool/queued/ratelimit", nil) // Dropped due to rate limiting
	queuedNofundsCounter   = metrics.NewRegisteredCounter("txpool/queued/nofunds", nil)   // Dropped due to out-of-funds

	// General tx metrics
	invalidTxCounter     = metrics.NewRegisteredCounter("txpool/invalid", nil)
	underpricedTxCounter = metrics.NewRegisteredCounter("txpool/underpriced", nil)
)

// TxStatus is the current status of a transaction as seen by the pool.
type TxStatus uint

// Constants for TxStatus.
const (
	TxStatusUnknown TxStatus = iota
	TxStatusQueued
	TxStatusPending
	TxStatusIncluded
)

// blockChain provides the state of blockchain and current gas limit to do
// some pre checks in tx pool and event subscribers.
type blockChain interface {
	CurrentBlock() *types.Block
	GetBlock(hash common.Hash, number uint64) *types.Block
	StateAt(root common.Hash) (*state.DB, error)

	SubscribeChainHeadEvent(ch chan<- ChainHeadEvent) event.Subscription
}

// TxPoolConfig are the configuration parameters of the transaction pool.
type TxPoolConfig struct {
	Locals    []common.Address // Addresses that should be treated by default as local
	NoLocals  bool             // Whether local transaction handling should be disabled
	Journal   string           // Journal of local transactions to survive node restarts
	Rejournal time.Duration    // Time interval to regenerate the local transaction journal

	PriceLimit uint64 // Minimum gas price to enforce for acceptance into the pool
	PriceBump  uint64 // Minimum price bump percentage to replace an already existing transaction (nonce)

	AccountSlots uint64 // Number of executable transaction slots guaranteed per account
	GlobalSlots  uint64 // Maximum number of executable transaction slots for all accounts
	AccountQueue uint64 // Maximum number of non-executable transaction slots permitted per account
	GlobalQueue  uint64 // Maximum number of non-executable transaction slots for all accounts

	Lifetime time.Duration // Maximum amount of time non-executable transaction are queued

	Blacklist map[common.Address]struct{} // Set of accounts that cannot be a part of any transaction
}

// DefaultTxPoolConfig contains the default configurations for the transaction
// pool.
var DefaultTxPoolConfig = TxPoolConfig{
	Journal:   "transactions.rlp",
	Rejournal: time.Hour,

	PriceLimit: 1,
	PriceBump:  10,

	//AccountSlots: 16, // ori
	//GlobalSlots:  4096, // ori
	AccountSlots: 16384,
	GlobalSlots:  16384,
	// dynamic sharding
	// 修改每个账户可以执行的最多交易的队列
	//AccountQueue: 64, // ori
	AccountQueue: 16384,
	//GlobalQueue:  1024, // ori
	GlobalQueue:  16384,

	Lifetime: 30 * time.Minute,

	Blacklist: map[common.Address]struct{}{},
}

// sanitize checks the provided user configurations and changes anything that's
// unreasonable or unworkable.
func (config *TxPoolConfig) sanitize() TxPoolConfig {
	conf := *config
	if conf.Rejournal < time.Second {
		utils.Logger().Warn().
			Dur("provided", conf.Rejournal).
			Dur("updated", time.Second).
			Msg("Sanitizing invalid txpool journal time")
		conf.Rejournal = time.Second
	}
	if conf.PriceLimit < 1 {
		utils.Logger().Warn().
			Uint64("provided", conf.PriceLimit).
			Uint64("updated", DefaultTxPoolConfig.PriceLimit).
			Msg("Sanitizing invalid txpool price limit")
		conf.PriceLimit = DefaultTxPoolConfig.PriceLimit
	}
	if conf.PriceBump < 1 {
		utils.Logger().Warn().
			Uint64("provided", conf.PriceBump).
			Uint64("updated", DefaultTxPoolConfig.PriceBump).
			Msg("Sanitizing invalid txpool price bump")
		conf.PriceBump = DefaultTxPoolConfig.PriceBump
	}
	if conf.Blacklist == nil {
		utils.Logger().Warn().Msg("Sanitizing nil blacklist set")
		conf.Blacklist = DefaultTxPoolConfig.Blacklist
	}

	return conf
}

// TxPool contains all currently known transactions. Transactions
// enter the pool when they are received from the network or submitted
// locally. They exit the pool when they are included in the blockchain.
//
// The pool separates processable transactions (which can be applied to the
// current state) and future transactions. Transactions move between those
// two states over time as they are received and processed.
type TxPool struct {
	config       TxPoolConfig
	chainconfig  *params.ChainConfig
	chain        blockChain
	gasPrice     *big.Int
	txFeed       event.Feed
	scope        event.SubscriptionScope
	chainHeadCh  chan ChainHeadEvent     // txpool订阅区块头的消息
	chainHeadSub event.Subscription      // 区块头消息订阅器，通过它可以取消消息
	mu           sync.RWMutex // 用的是读写锁

	currentState  *state.DB           // Current state in the blockchain head // 当前头区块对应的状态
	pendingState  *state.ManagedState // Pending state tracking virtual nonces  假设pending列表最后一个交易执行完后应该的状态
	currentMaxGas uint64              // Current gas limit for transaction caps

	locals  *accountSet // Set of local transaction to exempt from eviction rules
	journal *txJournal  // Journal of local transaction to back up to disk

	pending map[common.Address]*txList   // All currently processable transactions  其中key是交易发起者的地址，按nonce排序
	queue   map[common.Address]*txList   // Queued but non-processable transactions
	beats   map[common.Address]time.Time // Last heartbeat from each known account
	all     *txLookup                    // All transactions to allow lookups
	priced  *txPricedList                // All transactions sorted by price

	wg sync.WaitGroup // for shutdown sync

	txErrorSink *types.TransactionErrorSink // All failed txs gets reported here

	homestead bool
	istanbul  bool

	// 计算TPS
	tpsStartTime time.Time
	lastAccumulateTime float64
}

// NewTxPool creates a new transaction pool to gather, sort and filter inbound
// transactions from the network.
func NewTxPool(config TxPoolConfig, chainconfig *params.ChainConfig,
	chain blockChain, txErrorSink *types.TransactionErrorSink,
) *TxPool {
	// Sanitize the input to ensure no vulnerable gas prices are set
	config = (&config).sanitize()

	// Create the transaction pool with its initial settings
	pool := &TxPool{
		config:      config,
		chainconfig: chainconfig,
		chain:       chain,
		pending:     make(map[common.Address]*txList),
		queue:       make(map[common.Address]*txList),
		beats:       make(map[common.Address]time.Time),
		all:         newTxLookup(),
		chainHeadCh: make(chan ChainHeadEvent, chainHeadChanSize),
		gasPrice:    new(big.Int).SetUint64(config.PriceLimit),
		txErrorSink: txErrorSink,
	}
	pool.locals = newAccountSet(chainconfig.ChainID)
	for _, addr := range config.Locals {
		utils.Logger().Info().Interface("address", addr).Msg("Setting new local account")
		pool.locals.add(addr)
	}
	pool.priced = newTxPricedList(pool.all)
	pool.reset(nil, chain.CurrentBlock().Header())

	// If local transactions and journaling is enabled, load from disk
	if !config.NoLocals && config.Journal != "" {
		pool.journal = newTxJournal(config.Journal)

		if err := pool.journal.load(pool.AddLocals); err != nil {
			utils.Logger().Warn().Err(err).Msg("Failed to load transaction journal")
		}
		if err := pool.journal.rotate(pool.local()); err != nil {
			utils.Logger().Warn().Err(err).Msg("Failed to rotate transaction journal")
		}
	}
	// Subscribe events from blockchain 订阅规范链更新事件
	pool.chainHeadSub = pool.chain.SubscribeChainHeadEvent(pool.chainHeadCh)

	// Start the event loop and return 启动事件监听
	pool.wg.Add(1)
	go pool.loop()

	return pool
}

// loop is the transaction pool's main event loop, waiting for and reacting to
// outside blockchain events as well as for various reporting and transaction
// eviction events. 监听事件
func (pool *TxPool) loop() {
	defer pool.wg.Done()

	// Start the stats reporting and transaction eviction tickers
	var prevPending, prevQueued, prevStales int
	// report为每8秒钟报告一次状态  8 * time.Second
	report := time.NewTicker(statsReportInterval)
	defer report.Stop()
	// evict为每分钟检测不活动account的交易移除 time.Minute
	evict := time.NewTicker(evictionInterval)
	defer evict.Stop()
	// journal为每小时更新一下本地的local transaction journal
	journal := time.NewTicker(pool.config.Rejournal)
	defer journal.Stop()

	// Track the previous head headers for transaction reorgs
	head := pool.chain.CurrentBlock()

	// Keep waiting for and reacting to the various events
	for {
		select {
		// Handle ChainHeadEvent 监听规范链更新事件，重置交易池：pool.reset()；
		case ev := <-pool.chainHeadCh:
			// block最终插入区块链时，记录时间
			// 1. 第一个区块插入的时候，计算开始时间
			// 2. 计算时间duration，累加时间
			if pool.chain.CurrentBlock().NumberU64() == uint64(1){
				pool.tpsStartTime = time.Now()
				pool.lastAccumulateTime = 0
			}else{
				// 计算TPS
				tpsDuration := time.Since(pool.tpsStartTime)
				// 计算秒
				sumDuration := tpsDuration.Seconds()
				duration := sumDuration-pool.lastAccumulateTime
				txNumPerBlock := len(pool.chain.CurrentBlock().Transactions())
				//tps := float64(uint64(txNumPerBlock) * (pool.chain.CurrentBlock().NumberU64()-1)) / duration
				// 只计算当前区块带来的TPS
				tps := float64(txNumPerBlock) / duration
				//fmt.Printf("tps: %f, duration: %f, accumulateTime: %f, blockNum: %d", tps, duration, sumDuration, pool.chain.CurrentBlock().NumberU64())
				utils.Logger().Info().
					Float64("tps", tps).
					Float64("duration", duration).
					Float64("accumulateTime", sumDuration).
					Uint64("blockNum", pool.chain.CurrentBlock().NumberU64()).
					Msg("=============Throughput=============")
				// 最后重新给上一轮的累计时间赋值
				pool.lastAccumulateTime = sumDuration
			}
			// END

			if ev.Block != nil {
				pool.mu.Lock()
				if pool.chainconfig.IsS3(ev.Block.Epoch()) {
					pool.homestead = true
				}
				if pool.chainconfig.IsIstanbul(ev.Block.Epoch()) {
					pool.istanbul = true
				}
				pool.reset(head.Header(), ev.Block.Header())
				head = ev.Block
				pool.mu.Unlock()
			}
		// Be unsubscribed due to system stopped
		case <-pool.chainHeadSub.Err():
			return

		// Handle stats reporting ticks 监听report.C，定时打印最新pending和queue状态；
		case <-report.C:
			pool.mu.RLock()
			pending, queued := pool.stats()
			stales := pool.priced.stales
			pool.mu.RUnlock()

			if pending != prevPending || queued != prevQueued || stales != prevStales {
				utils.Logger().Debug().
					Int("executable", pending).
					Int("queued", queued).
					Int("stales", stales).
					Msg("Transaction pool status report")
				prevPending, prevQueued, prevStales = pending, queued, stales
			}

		// Handle inactive account transaction eviction 监听evict.C，定时删除超时交易；
		case <-evict.C:
			pool.mu.Lock()
			for addr := range pool.queue {
				// Skip local transactions from the eviction mechanism
				if pool.locals.contains(addr) {
					continue
				}
				// Any non-locals old enough should be removed
				if time.Since(pool.beats[addr]) > pool.config.Lifetime {
					b32addr, err := hmyCommon.AddressToBech32(addr)
					if err != nil {
						b32addr = "unknown"
					}
					for _, tx := range pool.queue[addr].Flatten() {
						pool.removeTx(tx.Hash(), true)
						pool.txErrorSink.Add(tx, fmt.Errorf("removed transaction for inactive account %v", b32addr))
					}
				}
			}
			pool.mu.Unlock()

		// Handle local transaction journal rotation 监听journal.C，定时本地储存尚未额打包的local交易；
		case <-journal.C:
			if pool.journal != nil {
				pool.mu.Lock()
				if err := pool.journal.rotate(pool.local()); err != nil {
					utils.Logger().Warn().Err(err).Msg("Failed to rotate local tx journal")
				}
				pool.mu.Unlock()
			}
		}
	}
}

// lockedReset is a wrapper around reset to allow calling it in a thread safe
// manner. This method is only ever used in the tester! 只在测试场景下使用
func (pool *TxPool) lockedReset(oldHead, newHead *block.Header) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.reset(oldHead, newHead)
}

// reset retrieves the current state of the blockchain and ensures the content
// of the transaction pool is valid with regard to the chain state.
// 输入最新header，重置交易池，重要方法
// 功能流程：
// 1）找到由于规范链更新而作废的交易；
// 2）给交易池设置最新的世界状态；
// 3）把旧链退回的交易重新放入交易池；
// 4）由于分叉引起的降级，将pending中部分交易移到queue里面；
// 5）由于规范链更新，将queue里面部分交易升级移到pending里。
func (pool *TxPool) reset(oldHead, newHead *block.Header) {
	// If we're reorging an old state, reinject all dropped transactions
	var reinject types.PoolTransactions

	// 第一，找到由于规范链更新而作废的交易
	// 新区快头的父区块不等于老区块，说明新老区块不在同一条链
	if oldHead != nil && oldHead.Hash() != newHead.ParentHash() {
		// If the reorg is too deep, avoid doing it (will happen during fast sync)
		oldNum := oldHead.Number().Uint64()
		newNum := newHead.Number().Uint64()

		// 如果新头区块和旧头区块相差大于64，则所有交易不必回退到交易池
		if depth := uint64(math.Abs(float64(oldNum) - float64(newNum))); depth > 64 {
			utils.Logger().Debug().Uint64("depth", depth).Msg("Skipping deep transaction reorg")
		} else {
			// Reorg seems shallow enough to pull in all transactions into memory
			var discarded, included types.PoolTransactions

			var (
				rem = pool.chain.GetBlock(oldHead.Hash(), oldHead.Number().Uint64())
				add = pool.chain.GetBlock(newHead.Hash(), newHead.Number().Uint64())
			)
			for rem.NumberU64() > add.NumberU64() {
				for _, tx := range rem.Transactions() {
					discarded = append(discarded, tx)
				}
				for _, tx := range rem.StakingTransactions() {
					discarded = append(discarded, tx)
				}
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					utils.Logger().Error().
						Str("block", oldHead.Number().String()).
						Str("hash", oldHead.Hash().Hex()).
						Msg("Unrooted old chain seen by tx pool")
					return
				}
			}
			for add.NumberU64() > rem.NumberU64() {
				for _, tx := range add.Transactions() {
					included = append(included, tx)
				}
				for _, tx := range add.StakingTransactions() {
					included = append(included, tx)
				}
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					utils.Logger().Error().
						Str("block", newHead.Number().String()).
						Str("hash", newHead.Hash().Hex()).
						Msg("Unrooted new chain seen by tx pool")
					return
				}
			}
			for rem.Hash() != add.Hash() {
				for _, tx := range rem.Transactions() {
					discarded = append(discarded, tx)
				}
				for _, tx := range rem.StakingTransactions() {
					discarded = append(discarded, tx)
				}
				if rem = pool.chain.GetBlock(rem.ParentHash(), rem.NumberU64()-1); rem == nil {
					utils.Logger().Error().
						Str("block", oldHead.Number().String()).
						Str("hash", oldHead.Hash().Hex()).
						Msg("Unrooted old chain seen by tx pool")
					return
				}
				for _, tx := range add.Transactions() {
					included = append(included, tx)
				}
				for _, tx := range add.StakingTransactions() {
					included = append(included, tx)
				}
				if add = pool.chain.GetBlock(add.ParentHash(), add.NumberU64()-1); add == nil {
					utils.Logger().Error().
						Str("block", newHead.Number().String()).
						Str("hash", newHead.Hash().Hex()).
						Msg("Unrooted new chain seen by tx pool")
					return
				}
			}
			reinject = types.PoolTxDifference(discarded, included)
		}
	}
	// Initialize the internal state to the current head
	if newHead == nil {
		newHead = pool.chain.CurrentBlock().Header() // Special case during testing
	}
	// 第二，给交易池设置最新的世界状态
	statedb, err := pool.chain.StateAt(newHead.Root())
	if err != nil {
		utils.Logger().Error().Err(err).Msg("Failed to reset txpool state")
		return
	}
	// 设置新链头区块的状态
	pool.currentState = statedb
	pool.pendingState = state.ManageState(statedb)
	pool.currentMaxGas = newHead.GasLimit()

	// 第三，把旧链回退的交易放入交易池
	// Inject any transactions discarded due to reorgs
	utils.Logger().Debug().Int("count", len(reinject)).Msg("Reinjecting stale transactions")
	//senderCacher.recover(pool.signer, reinject)
	pool.addTxsLocked(reinject, false)

	// validate the pool of pending transactions, this will remove
	// any transactions that have been included in the block or
	// have been invalidated because of another transaction (e.g.
	// higher gas price)
	// 第四，交易降级
	pool.demoteUnexecutables(newHead.Number().Uint64())

	// Update all accounts to the latest known pending nonce
	// 把所有的accounts的nonce更新到最新的pending Nonce
	for addr, list := range pool.pending {
		txs := list.Flatten() // Heavy but will be cached and is needed by the miner anyway
		pool.pendingState.SetNonce(addr, txs[len(txs)-1].Nonce()+1)
	}
	// Check the queue and move transactions over to the pending if possible
	// or remove those that have become invalid
	// 第五，交易升级
	pool.promoteExecutables(nil)
}

// GetTxPoolSize returns tx pool size.
func (pool *TxPool) GetTxPoolSize() uint64 {
	return uint64(len(pool.pending)) + uint64(len(pool.queue))
}

// Stop terminates the transaction pool.
func (pool *TxPool) Stop() {
	// Unsubscribe all subscriptions registered from txpool
	pool.scope.Close()

	// Unsubscribe subscriptions registered from blockchain
	pool.chainHeadSub.Unsubscribe()
	pool.wg.Wait()

	if pool.journal != nil {
		pool.journal.close()
	}
	utils.Logger().Info().Msg("Transaction pool stopped")
}

// SubscribeNewTxsEvent registers a subscription of NewTxsEvent and
// starts sending event to the given channel.
func (pool *TxPool) SubscribeNewTxsEvent(ch chan<- NewTxsEvent) event.Subscription {
	return pool.scope.Track(pool.txFeed.Subscribe(ch))
}

// GasPrice returns the current gas price enforced by the transaction pool.
func (pool *TxPool) GasPrice() *big.Int {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return new(big.Int).Set(pool.gasPrice)
}

// SetGasPrice updates the minimum price required by the transaction pool for a
// new transaction, and drops all transactions below this threshold.
func (pool *TxPool) SetGasPrice(price *big.Int) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pool.gasPrice = price
	for _, tx := range pool.priced.Cap(price, pool.locals) {
		pool.removeTx(tx.Hash(), false)
		pool.txErrorSink.Add(tx,
			fmt.Errorf("dropped transaction below new gas price threshold of %v", price.String()))
	}
	utils.Logger().Info().Str("price", price.String()).Msg("Transaction pool price threshold updated")
}

// State returns the virtual managed state of the transaction pool.
func (pool *TxPool) State() *state.ManagedState {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.pendingState
}

// Stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) Stats() (int, int) {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	return pool.stats()
}

// stats retrieves the current pool stats, namely the number of pending and the
// number of queued (non-executable) transactions.
func (pool *TxPool) stats() (int, int) {
	pending := 0
	for _, list := range pool.pending {
		pending += list.Len()
	}
	queued := 0
	for _, list := range pool.queue {
		queued += list.Len()
	}
	return pending, queued
}

// Content retrieves the data content of the transaction pool, returning all the
// pending as well as queued transactions, grouped by account and sorted by nonce.
func (pool *TxPool) Content() (map[common.Address]types.PoolTransactions, map[common.Address]types.PoolTransactions) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.PoolTransactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	queued := make(map[common.Address]types.PoolTransactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return pending, queued
}

// Pending retrieves all currently executable transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Pending() (map[common.Address]types.PoolTransactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := make(map[common.Address]types.PoolTransactions)
	for addr, list := range pool.pending {
		pending[addr] = list.Flatten()
	}
	return pending, nil
}

// Queued retrieves all currently non-executable transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) Queued() (map[common.Address]types.PoolTransactions, error) {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	queued := make(map[common.Address]types.PoolTransactions)
	for addr, list := range pool.queue {
		queued[addr] = list.Flatten()
	}
	return queued, nil
}

// Locals retrieves the accounts currently considered local by the pool.
func (pool *TxPool) Locals() []common.Address {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.locals.flatten()
}

// local retrieves all currently known local transactions, grouped by origin
// account and sorted by nonce. The returned transaction set is a copy and can be
// freely modified by calling code.
func (pool *TxPool) local() map[common.Address]types.PoolTransactions {
	txs := make(map[common.Address]types.PoolTransactions)
	for addr := range pool.locals.accounts {
		if pending := pool.pending[addr]; pending != nil {
			txs[addr] = append(txs[addr], pending.Flatten()...)
		}
		if queued := pool.queue[addr]; queued != nil {
			txs[addr] = append(txs[addr], queued.Flatten()...)
		}
	}
	return txs
}

// validateTx checks whether a transaction is valid according to the consensus
// rules and adheres to some heuristic limits of the local node (price and size).
// 验证进入交易池的交易
func (pool *TxPool) validateTx(tx types.PoolTransaction, local bool) error {
	// 1. 交易的shard必须和当前区块的shard相同
	if tx.ShardID() != pool.chain.CurrentBlock().ShardID() {
		return errors.WithMessagef(ErrInvalidShard, "transaction shard is %d", tx.ShardID())
	}
	/* dynamic sharding */
	// 判断账户是否是freeze状态, 如果from和to有任意一方账户是freeze状态，则返回错误
	fromAddr, _ := tx.SenderAddress()
	if pool.currentState.GetFreezeState(fromAddr) || pool.currentState.GetFreezeState(*tx.To()){
		return errors.WithMessagef(ErrAddrHasFreezed, "address is freezed")
	}
	// For DOS prevention, reject excessively large transactions.
	// 2. 交易大小必须小于 MaxPoolTransactionDataSize = 1280 * 1024 = 1.25Mb
	if tx.Size() >= types.MaxPoolTransactionDataSize {
		return errors.WithMessagef(ErrOversizedData, "transaction size is %s", tx.Size().String())
	}
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur if you create a transaction using the RPC.
	// 3. 交易金额必须非负值
	if tx.Value().Sign() < 0 {
		return errors.WithMessagef(ErrNegativeValue, "transaction value is %s", tx.Value().String())
	}
	// Ensure the transaction doesn't exceed the current block limit gas.
	// 4. 交易的gasLimit必须 < 交易池当前规定最大gas
	if pool.currentMaxGas < tx.GasLimit() {
		return errors.WithMessagef(ErrGasLimit, "transaction gas is %d", tx.GasLimit())
	}
	// Make sure the transaction is signed properly
	// 5. 交易签名必须有效
	from, err := tx.SenderAddress()
	if err != nil {
		if b32, err := hmyCommon.AddressToBech32(from); err == nil {
			return errors.WithMessagef(ErrInvalidSender, "transaction sender is %s", b32)
		}
		return ErrInvalidSender
	}
	// Make sure transaction does not have blacklisted addresses
	// 6. 交易中的发送地址不在blacklist上
	if _, exists := (pool.config.Blacklist)[from]; exists {
		if b32, err := hmyCommon.AddressToBech32(from); err == nil {
			return errors.WithMessagef(ErrBlacklistFrom, "transaction sender is %s", b32)
		}
		return ErrBlacklistFrom
	}
	// Make sure transaction does not burn funds by sending funds to blacklisted address
	// 7. 交易接收地址不在blacklist上
	if tx.To() != nil {
		if _, exists := (pool.config.Blacklist)[*tx.To()]; exists {
			if b32, err := hmyCommon.AddressToBech32(*tx.To()); err == nil {
				return errors.WithMessagef(ErrBlacklistTo, "transaction receiver is %s", b32)
			}
			return ErrBlacklistTo
		}
	}
	// Drop non-local transactions under our own minimal accepted gas price
	// 8. 交易的gas price必须大于交易池设置的gas price
	local = local || pool.locals.contains(from) // account may be local even if the transaction arrived from the network
	if !local && pool.gasPrice.Cmp(tx.GasPrice()) > 0 {
		gasPrice := new(big.Float).SetInt64(tx.GasPrice().Int64())
		gasPrice = gasPrice.Mul(gasPrice, new(big.Float).SetFloat64(1e-9)) // Gas-price is in Nano
		return errors.WithMessagef(ErrUnderpriced, "transaction gas-price is %.18f ONE", gasPrice)
	}
	// Ensure the transaction adheres to nonce ordering
	// 9. 交易的Nonce值必须大于链上该账户的Nonce
	if pool.currentState.GetNonce(from) > tx.Nonce() {
		return errors.WithMessagef(ErrNonceTooLow, "transaction nonce is %d", tx.Nonce())
	}
	// Transactor should have enough funds to cover the costs
	// cost == V + GP * GL
	// 10. 交易账户余额必须 > 交易额 + gasPrice * gasLimit
	cost, err := tx.Cost()
	if err != nil {
		return err
	}
	stakingTx, isStakingTx := tx.(*staking.StakingTransaction)
	if !isStakingTx || (isStakingTx && stakingTx.StakingType() != staking.DirectiveDelegate) {
		if pool.currentState.GetBalance(from).Cmp(cost) < 0 {
			return errors.Wrapf(
				ErrInsufficientFunds,
				"current shard-id: %d",
				pool.chain.CurrentBlock().ShardID(),
			)
		}
	}
	// 11. 交易的gasLimit必须 > 对应数据量所需要的最低gas水平
	intrGas := uint64(0)
	if isStakingTx {
		intrGas, err = IntrinsicGas(tx.Data(), false, pool.homestead, pool.istanbul, stakingTx.StakingType() == staking.DirectiveCreateValidator)
	} else {
		intrGas, err = IntrinsicGas(tx.Data(), tx.To() == nil, pool.homestead, pool.istanbul, false)
	}
	if err != nil {
		return err
	}
	if tx.GasLimit() < intrGas {
		return errors.WithMessagef(ErrIntrinsicGas, "transaction gas is %d", tx.GasLimit())
	}
	// Do more checks if it is a staking transaction
	// 12. 如果是staking交易，则需要更多检查
	if isStakingTx {
		return pool.validateStakingTx(stakingTx)
	}
	return nil
}

// validateStakingTx checks the staking message based on the staking directive
func (pool *TxPool) validateStakingTx(tx *staking.StakingTransaction) error {
	// from address already validated
	from, _ := tx.SenderAddress()
	b32, _ := hmyCommon.AddressToBech32(from)

	switch tx.StakingType() {
	case staking.DirectiveCreateValidator:
		msg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCreateValidator)
		if err != nil {
			return err
		}
		stkMsg, ok := msg.(*staking.CreateValidator)
		if !ok {
			return ErrInvalidMsgForStakingDirective
		}
		if from != stkMsg.ValidatorAddress {
			return errors.WithMessagef(ErrInvalidSender, "staking transaction sender is %s", b32)
		}
		currentBlockNumber := pool.chain.CurrentBlock().Number()
		pendingBlockNumber := new(big.Int).Add(currentBlockNumber, big.NewInt(1))
		pendingEpoch := pool.chain.CurrentBlock().Epoch()
		if shard.Schedule.IsLastBlock(currentBlockNumber.Uint64()) {
			pendingEpoch = new(big.Int).Add(pendingEpoch, big.NewInt(1))
		}
		chainContext, ok := pool.chain.(ChainContext)
		if !ok {
			chainContext = nil // might use testing blockchain, set to nil for verifier to handle.
		}
		_, err = VerifyAndCreateValidatorFromMsg(pool.currentState, chainContext, pendingEpoch, pendingBlockNumber, stkMsg)
		return err
	case staking.DirectiveEditValidator:
		msg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveEditValidator)
		if err != nil {
			return err
		}
		stkMsg, ok := msg.(*staking.EditValidator)
		if !ok {
			return ErrInvalidMsgForStakingDirective
		}
		if from != stkMsg.ValidatorAddress {
			return errors.WithMessagef(ErrInvalidSender, "staking transaction sender is %s", b32)
		}
		chainContext, ok := pool.chain.(ChainContext)
		if !ok {
			chainContext = nil // might use testing blockchain, set to nil for verifier to handle.
		}
		pendingBlockNumber := new(big.Int).Add(pool.chain.CurrentBlock().Number(), big.NewInt(1))

		_, err = VerifyAndEditValidatorFromMsg(
			pool.currentState, chainContext,
			pool.chain.CurrentBlock().Epoch(),
			pendingBlockNumber, stkMsg,
		)
		return err
	case staking.DirectiveDelegate:
		msg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveDelegate)
		if err != nil {
			return err
		}
		stkMsg, ok := msg.(*staking.Delegate)
		if !ok {
			return ErrInvalidMsgForStakingDirective
		}
		if from != stkMsg.DelegatorAddress {
			return errors.WithMessagef(ErrInvalidSender, "staking transaction sender is %s", b32)
		}

		chain, ok := pool.chain.(ChainContext)
		if !ok {
			utils.Logger().Debug().Msg("Missing chain context in txPool")
			return nil // for testing, chain could be testing blockchain
		}
		delegations, err := chain.ReadDelegationsByDelegator(stkMsg.DelegatorAddress)
		if err != nil {
			return err
		}
		pendingEpoch := pool.pendingEpoch()
		_, delegateAmt, _, err := VerifyAndDelegateFromMsg(
			pool.currentState, pendingEpoch, stkMsg, delegations, pool.chainconfig)
		if err != nil {
			return err
		}
		// We need to deduct gas price and verify balance since txn.Cost() is not accurate for delegate
		// staking transaction because of re-delegation.
		gasAmt := new(big.Int).Mul(tx.GasPrice(), new(big.Int).SetUint64(tx.GasLimit()))
		totalAmt := new(big.Int).Add(delegateAmt, gasAmt)
		if bal := pool.currentState.GetBalance(from); bal.Cmp(totalAmt) < 0 {
			return fmt.Errorf("not enough balance for delegation: %v < %v", bal, delegateAmt)
		}
		return nil
	case staking.DirectiveUndelegate:
		msg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveUndelegate)
		if err != nil {
			return err
		}
		stkMsg, ok := msg.(*staking.Undelegate)
		if !ok {
			return ErrInvalidMsgForStakingDirective
		}
		if from != stkMsg.DelegatorAddress {
			return errors.WithMessagef(ErrInvalidSender, "staking transaction sender is %s", b32)
		}

		_, err = VerifyAndUndelegateFromMsg(pool.currentState, pool.pendingEpoch(), stkMsg)
		return err
	case staking.DirectiveCollectRewards:
		msg, err := staking.RLPDecodeStakeMsg(tx.Data(), staking.DirectiveCollectRewards)
		if err != nil {
			return err
		}
		stkMsg, ok := msg.(*staking.CollectRewards)
		if !ok {
			return ErrInvalidMsgForStakingDirective
		}
		if from != stkMsg.DelegatorAddress {
			return errors.WithMessagef(ErrInvalidSender, "staking transaction sender is %s", b32)
		}
		chain, ok := pool.chain.(ChainContext)
		if !ok {
			utils.Logger().Debug().Msg("Missing chain context in txPool")
			return nil // for testing, chain could be testing blockchain
		}
		delegations, err := chain.ReadDelegationsByDelegator(stkMsg.DelegatorAddress)
		if err != nil {
			return err
		}

		_, _, err = VerifyAndCollectRewardsFromDelegation(pool.currentState, delegations)
		return err
	default:
		return staking.ErrInvalidStakingKind
	}
}

func (pool *TxPool) pendingEpoch() *big.Int {
	currentBlock := pool.chain.CurrentBlock()
	pendingEpoch := currentBlock.Epoch()
	if shard.Schedule.IsLastBlock(currentBlock.Number().Uint64()) {
		pendingEpoch.Add(pendingEpoch, big.NewInt(1))
	}
	return pendingEpoch
}

// add validates a transaction and inserts it into the non-executable queue for
// later pending promotion and execution. If the transaction is a replacement for
// an already pending or queued one, it overwrites the previous and returns this
// so outer code doesn't uselessly call promote.
//
// If a newly added transaction is marked as local, its sending account will be
// whitelisted, preventing any associated transaction from being dropped out of
// the pool due to pricing constraints.
// 向交易池添加交易
func (pool *TxPool) add(tx types.PoolTransaction, local bool) (bool, error) {
	logger := utils.Logger().With().Stack().Logger()
	// If the transaction is in the error sink, remove it as it may succeed
	if pool.txErrorSink.Contains(tx.Hash().String()) {
		pool.txErrorSink.Remove(tx)
	}
	// If the transaction is already known, discard it
	hash := tx.Hash()
	if pool.all.Get(hash) != nil {
		logger.Info().Str("hash", hash.Hex()).Msg("Discarding already known transaction")
		return false, errors.WithMessagef(ErrKnownTransaction, "transaction hash %x", hash)
	}
	// If the transaction fails basic validation, discard it
	// 新增对statetransfer交易的验证？
	if err := pool.validateTx(tx, local); err != nil {
		logger.Warn().Err(err).Str("hash", hash.Hex()).Msg("Discarding invalid transaction")
		invalidTxCounter.Inc(1)
		return false, err
	}
	// If the transaction pool is full, discard underpriced transactions
	if uint64(pool.all.Count()) >= pool.config.GlobalSlots+pool.config.GlobalQueue {
		// If the new transaction is underpriced, don't accept it
		if !local && pool.priced.Underpriced(tx, pool.locals) {
			gasPrice := new(big.Float).SetInt64(tx.GasPrice().Int64())
			gasPrice = gasPrice.Mul(gasPrice, new(big.Float).SetFloat64(1e-9)) // Gas-price is in Nano
			logger.Warn().
				Str("hash", hash.Hex()).
				Str("price", tx.GasPrice().String()).
				Msg("Discarding underpriced transaction")
			underpricedTxCounter.Inc(1)
			return false, errors.WithMessagef(ErrUnderpriced, "transaction gas-price is %.18f ONE in full transaction pool", gasPrice)
		}
		// New transaction is better than our worse ones, make room for it
		drop := pool.priced.Discard(pool.all.Count()-int(pool.config.GlobalSlots+pool.config.GlobalQueue-1), pool.locals)
		for _, tx := range drop {
			gasPrice := new(big.Float).SetInt64(tx.GasPrice().Int64())
			gasPrice = gasPrice.Mul(gasPrice, new(big.Float).SetFloat64(1e-9)) // Gas-price is in Nano
			pool.removeTx(tx.Hash(), false)
			underpricedTxCounter.Inc(1)
			pool.txErrorSink.Add(tx,
				errors.WithMessagef(ErrUnderpriced, "transaction gas-price is %.18f ONE in full transaction pool", gasPrice))
			logger.Warn().
				Str("hash", tx.Hash().Hex()).
				Str("price", tx.GasPrice().String()).
				Msg("Discarding freshly underpriced transaction")
		}
	}
	// If the transaction is replacing an already pending one, do directly
	from, _ := tx.SenderAddress() // already validated

	// dynamic sharding
	// list.Overlaps(tx)判断交易的Nonce是否重复
	if list := pool.pending[from]; list != nil && list.Overlaps(tx) {
		// Nonce already pending, check if required price bump is met
		inserted, old := list.Add(tx, pool.config.PriceBump)
		if !inserted {
			pendingDiscardCounter.Inc(1)
			return false, errors.WithMessage(ErrReplaceUnderpriced, "existing transaction price was not bumped enough")
		}
		// New transaction is better, replace old one
		if old != nil {
			pool.all.Remove(old.Hash())
			pool.priced.Removed()
			pendingReplaceCounter.Inc(1)
			pool.txErrorSink.Add(old,
				fmt.Errorf("replaced transaction, new transaction %v has same nonce & higher price", tx.Hash().String()))
			logger.Info().
				Str("hash", old.Hash().String()).
				Str("new-tx-hash", tx.Hash().String()).
				Str("price", old.GasPrice().String()).
				Msg("Replaced transaction")
		}
		pool.all.Add(tx)
		pool.priced.Put(tx)
		pool.journalTx(from, tx)

		// Set or refresh beat for account timeout eviction
		pool.beats[from] = time.Now()

		logger.Info().
			Str("hash", tx.Hash().Hex()).
			Interface("from", from).
			Interface("to", tx.To()).
			Str("price", tx.GasPrice().String()).
			Msg("Pooled new executable transaction")

		// We've directly injected a replacement transaction, notify subsystems
		// go pool.txFeed.Send(NewTxsEvent{types.PoolTransactions{tx}})

		return old != nil, nil
	}

	// New transaction isn't replacing a pending one, push into queue
	// 所以新交易如果不是替换pending里交易的，都是先添加到queue，再升级到pending？
	replace, err := pool.enqueueTx(tx)
	if err != nil {
		return false, err
	}
	// Mark local addresses and journal local transactions
	if local {
		if !pool.locals.contains(from) {
			utils.Logger().Info().Interface("address", from).Msg("Setting new local account")
			pool.locals.add(from)
		}
	}
	pool.journalTx(from, tx)

	// Set or refresh beat for account timeout eviction
	pool.beats[from] = time.Now()
	// dynamic sharding 注释这句，log太多
	/*
	logger.Info().
		Str("hash", hash.Hex()).
		Interface("from", from).
		Interface("to", tx.To()).
		Msg("Pooled new future transaction")
	 */
	return replace, nil
}

// enqueueTx inserts a new transaction into the non-executable transaction queue.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) enqueueTx(tx types.PoolTransaction) (bool, error) {
	// Try to insert the transaction into the future queue
	from, _ := tx.SenderAddress() // already validated
	if pool.queue[from] == nil {
		pool.queue[from] = newTxList(false)
	}
	inserted, old := pool.queue[from].Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		queuedDiscardCounter.Inc(1)
		return false, ErrReplaceUnderpriced
	}
	// Discard any previous transaction and mark this
	if old != nil {
		pool.all.Remove(old.Hash())
		pool.priced.Removed()
		queuedReplaceCounter.Inc(1)
		pool.txErrorSink.Add(old,
			fmt.Errorf("replaced enqueued non-executable transaction, new transaction %v has same nonce & higher price", tx.Hash().String()))
		utils.Logger().Info().
			Str("hash", old.Hash().String()).
			Str("new-tx-hash", tx.Hash().String()).
			Str("price", old.GasPrice().String()).
			Msg("Replaced enqueued non-executable transaction")
	}
	if pool.all.Get(tx.Hash()) == nil {
		pool.all.Add(tx)
		pool.priced.Put(tx)
	}
	return old != nil, nil
}

// journalTx adds the specified transaction to the local disk journal if it is
// deemed to have been sent from a local account.
func (pool *TxPool) journalTx(from common.Address, tx types.PoolTransaction) {
	// Only journal if it's enabled and the transaction is local
	if pool.journal == nil || !pool.locals.contains(from) {
		return
	}
	if err := pool.journal.insert(tx); err != nil {
		utils.Logger().Warn().Err(err).Msg("Failed to journal local transaction")
	}
}

// promoteTx adds a transaction to the pending (processable) list of transactions
// and returns whether it was inserted or an older was better.
//
// Note, this method assumes the pool lock is held!
func (pool *TxPool) promoteTx(addr common.Address, tx types.PoolTransaction) bool {
	// Try to insert the transaction into the pending queue
	if pool.pending[addr] == nil {
		pool.pending[addr] = newTxList(true)
	}
	list := pool.pending[addr]

	inserted, old := list.Add(tx, pool.config.PriceBump)
	if !inserted {
		// An older transaction was better, discard this
		pool.all.Remove(tx.Hash())
		pool.priced.Removed()
		pendingDiscardCounter.Inc(1)
		pool.txErrorSink.Add(tx, fmt.Errorf("could not promote to executable"))
		utils.Logger().Info().
			Str("hash", tx.Hash().String()).
			Msg("Could not promote to executable")
		return false
	}
	// Otherwise discard any previous transaction and mark this
	if old != nil {
		pool.all.Remove(old.Hash())
		pool.priced.Removed()
		pendingReplaceCounter.Inc(1)
		pool.txErrorSink.Add(old,
			fmt.Errorf("did not promote to executable, existing transaction %v has same nonce & higher price", tx.Hash().String()))
		utils.Logger().Info().
			Str("hash", old.Hash().String()).
			Str("existing-tx-hash", tx.Hash().String()).
			Msg("Did not promote to executable, new transaction has higher price")
	}
	// Failsafe to work around direct pending inserts (tests)
	if pool.all.Get(tx.Hash()) == nil {
		pool.all.Add(tx)
		pool.priced.Put(tx)
	}
	// Set the potentially new pending nonce and notify any subsystems of the new tx
	pool.beats[addr] = time.Now()
	pool.pendingState.SetNonce(addr, tx.Nonce()+1)

	return true
}

// AddLocal enqueues a single transaction into the pool if it is valid, marking
// the sender as a local one in the mean time, ensuring it goes around the local
// pricing constraints.
func (pool *TxPool) AddLocal(tx types.PoolTransaction) error {
	return pool.addTx(tx, !pool.config.NoLocals)
}

// AddRemote enqueues a single transaction into the pool if it is valid. If the
// sender is not among the locally tracked ones, full pricing constraints will
// apply.
func (pool *TxPool) AddRemote(tx types.PoolTransaction) error {
	return pool.addTx(tx, false)
}

// AddLocals enqueues a batch of transactions into the pool if they are valid,
// marking the senders as a local ones in the mean time, ensuring they go around
// the local pricing constraints.
func (pool *TxPool) AddLocals(txs types.PoolTransactions) []error {
	return pool.addTxs(txs, !pool.config.NoLocals)
}

// AddRemotes enqueues a batch of transactions into the pool if they are valid.
// If the senders are not among the locally tracked ones, full pricing constraints
// will apply.
func (pool *TxPool) AddRemotes(txs types.PoolTransactions) []error {
	return pool.addTxs(txs, false)
}

// addTx enqueues a single transaction into the pool if it is valid.
func (pool *TxPool) addTx(tx types.PoolTransaction, local bool) error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	// Try to inject the transaction and update any state
	replace, err := pool.add(tx, local)
	if err != nil {
		errCause := errors.Cause(err)
		// Ignore known transaction for tx rebroadcast case.
		if errCause != ErrKnownTransaction {
			pool.txErrorSink.Add(tx, err)
		}
		return errCause
	}
	// If we added a new transaction, run promotion checks and return
	// 如果添加了一笔新交易，执行promote check（这里去广播？）
	if !replace { // 没有替换掉pending中的交易，加入到queue，说明是新交易
		from, _ := tx.SenderAddress() // already validated
		pool.promoteExecutables([]common.Address{from})
	}
	return nil
}

// addTxs attempts to queue a batch of transactions if they are valid.
func (pool *TxPool) addTxs(txs types.PoolTransactions, local bool) []error {
	pool.mu.Lock()
	defer pool.mu.Unlock()

	return pool.addTxsLocked(txs, local)
}

// addTxsLocked attempts to queue a batch of transactions if they are valid,
// whilst assuming the transaction pool lock is already held.
func (pool *TxPool) addTxsLocked(txs types.PoolTransactions, local bool) []error {
	// Add the batch of transaction, tracking the accepted ones
	dirty := map[common.Address]struct{}{}
	errs := make([]error, txs.Len())

	for i, tx := range txs {
		replace, err := pool.add(tx, local)
		if err == nil && !replace {
			from, _ := tx.SenderAddress() // already validated
			dirty[from] = struct{}{}
		}
		errCause := errors.Cause(err)
		// Ignore known transaction for tx rebroadcast case.
		if err != nil && errCause != ErrKnownTransaction {
			pool.txErrorSink.Add(tx, err)
		}
		errs[i] = errCause
	}
	// Only reprocess the internal state if something was actually added
	if len(dirty) > 0 {
		addrs := make([]common.Address, len(dirty))
		i := 0
		for addr := range dirty {
			addrs[i] = addr
			i++
		}
		pool.promoteExecutables(addrs)
	}
	return errs
}

// Status returns the status (unknown/pending/queued) of a batch of transactions
// identified by their hashes.
func (pool *TxPool) Status(hashes []common.Hash) []TxStatus {
	pool.mu.RLock()
	defer pool.mu.RUnlock()

	status := make([]TxStatus, len(hashes))
	for i, hash := range hashes {
		if tx := pool.all.Get(hash); tx != nil {
			from, _ := tx.SenderAddress() // already validated
			if pool.pending[from] != nil && pool.pending[from].txs.items[tx.Nonce()] != nil {
				status[i] = TxStatusPending
			} else {
				status[i] = TxStatusQueued
			}
		}
	}
	return status
}

// Get returns a transaction if it is contained in the pool
// and nil otherwise.
func (pool *TxPool) Get(hash common.Hash) types.PoolTransaction {
	return pool.all.Get(hash)
}

// removeTx removes a single transaction from the queue, moving all subsequent
// transactions back to the future queue.
func (pool *TxPool) removeTx(hash common.Hash, outofbound bool) {
	// Fetch the transaction we wish to delete
	tx := pool.all.Get(hash)
	if tx == nil {
		return
	}
	addr, _ := tx.SenderAddress() // already validated during insertion

	// Remove it from the list of known transactions
	pool.all.Remove(hash)
	if outofbound {
		pool.priced.Removed()
	}
	// Remove the transaction from the pending lists and reset the account nonce
	if pending := pool.pending[addr]; pending != nil {
		if removed, invalids := pending.Remove(tx); removed {
			// If no more pending transactions are left, remove the list
			if pending.Empty() {
				delete(pool.pending, addr)
				delete(pool.beats, addr)
			}
			// Postpone any invalidated transactions
			for _, tx := range invalids {
				if _, err := pool.enqueueTx(tx); err != nil {
					pool.txErrorSink.Add(tx, err)
				}
			}
			// Update the account nonce if needed
			if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
				pool.pendingState.SetNonce(addr, nonce)
			}
			return
		}
	}
	// Transaction is in the future queue
	if future := pool.queue[addr]; future != nil {
		future.Remove(tx)
		if future.Empty() {
			delete(pool.queue, addr)
		}
	}
}

// promoteExecutables moves transactions that have become processable from the
// future queue to the set of pending transactions. During this process, all
// invalidated transactions (low nonce, low balance) are deleted.
// promoteExecutables将future queue中的交易移动到pending中
// 把给定的账号地址列表中已经变得可以执行的交易从queue列表中插入到pending中，同时检查一些失效的交易，然后发送交易池更新事件
func (pool *TxPool) promoteExecutables(accounts []common.Address) {
	// Track the promoted transactions to broadcast them at once
	var promoted types.PoolTransactions
	logger := utils.Logger().With().Stack().Logger()

	// Gather all the accounts potentially needing updates
	// 首先从queue中取出要升级交易的账户
	if accounts == nil {
		accounts = make([]common.Address, len(pool.queue))
		i := 0
		for addr := range pool.queue {
			accounts[i] = addr
			i++
		}
	}
	// Iterate over all accounts and promote any executable transactions
	// 从账户中取出交易
	for _, addr := range accounts {
		list := pool.queue[addr]
		if list == nil {
			continue // Just in case someone calls with a non existing account
		}
		// Drop all transactions that are deemed too old (low nonce)
		// 类似于降级中的方法，删除低于账户Nonce值的交易
		nonce := pool.currentState.GetNonce(addr)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.priced.Removed()
			logger.Debug().Str("hash", hash.Hex()).Msg("Removed old queued transaction")
			// Do not report to error sink as old txs are on chain or meaningful error caught elsewhere.
		}
		// Drop all transactions that are too costly (low balance or out of gas)
		// 过滤交易余额不足的交易或gas过大的交易，然后从all交易池中删除
		drops, errs, _ := list.FilterValid(pool, addr, 0)
		for i, tx := range drops {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.priced.Removed()
			queuedNofundsCounter.Inc(1)
			pool.txErrorSink.Add(tx, errs[i])
			logger.Warn().Str("hash", hash.Hex()).Err(errs[i]).
				Msg("Removed unpayable queued transaction")
		}
		// Gather all executable transactions and promote them
		// 收集所有可以被加入pending列表的交易，依据是交易序号连续且递增
		for _, tx := range list.Ready(pool.pendingState.GetNonce(addr)) {
			//hash := tx.Hash()
			if pool.promoteTx(addr, tx) {
				// dynamic sharding 注释这句，因为log中打的太多了
				//logger.Info().Str("hash", hash.Hex()).Msg("Promoting queued transaction")
				promoted = append(promoted, tx)
			}
		}
		// Drop all transactions over the allowed limit
		// 如果某个账户下的交易数量超过了阈值（默认64）,删除超过的交易
		if !pool.locals.contains(addr) {
			for _, tx := range list.Cap(int(pool.config.AccountQueue)) {
				hash := tx.Hash()
				pool.all.Remove(hash)
				pool.priced.Removed()
				queuedRateLimitCounter.Inc(1)
				pool.txErrorSink.Add(tx, fmt.Errorf("exceeds cap for queued transactions for account %s", addr.String()))
				logger.Warn().Str("hash", hash.Hex()).Msg("Removed cap-exceeding queued transaction")
			}
		}
		// Delete the entire queue entry if it became empty.
		// 如果当前账户为空，则删除该账户
		if list.Empty() {
			delete(pool.queue, addr)
		}
	}
	// Notify subsystem for new promoted transactions.
	//if len(promoted) > 0 {
	//	go pool.txFeed.Send(NewTxsEvent{promoted})
	//}
	// If the pending limit is overflown, start equalizing allowances
	pending := uint64(0)
	for _, list := range pool.pending {
		pending += uint64(list.Len())
	}
	if pending > pool.config.GlobalSlots {
		pendingBeforeCap := pending
		// Assemble a spam order to penalize large transactors first
		spammers := prque.New(nil)
		for addr, list := range pool.pending {
			// Only evict transactions from high rollers
			if !pool.locals.contains(addr) && uint64(list.Len()) > pool.config.AccountSlots {
				spammers.Push(addr, int64(list.Len()))
			}
		}
		// Gradually drop transactions from offenders
		offenders := []common.Address{}
		for pending > pool.config.GlobalSlots && !spammers.Empty() {
			// Retrieve the next offender if not local address
			offender, _ := spammers.Pop()
			offenders = append(offenders, offender.(common.Address))

			// Equalize balances until all the same or below threshold
			if len(offenders) > 1 {
				// Calculate the equalization threshold for all current offenders
				threshold := pool.pending[offender.(common.Address)].Len()

				// Iteratively reduce all offenders until below limit or threshold reached
				for pending > pool.config.GlobalSlots && pool.pending[offenders[len(offenders)-2]].Len() > threshold {
					for i := 0; i < len(offenders)-1; i++ {
						list := pool.pending[offenders[i]]
						for _, tx := range list.Cap(list.Len() - 1) {
							// Drop the transaction from the global pools too
							hash := tx.Hash()
							pool.all.Remove(hash)
							pool.priced.Removed()
							pool.txErrorSink.Add(tx, fmt.Errorf("fairness-exceeding pending transaction"))

							// Update the account nonce to the dropped transaction
							if nonce := tx.Nonce(); pool.pendingState.GetNonce(offenders[i]) > nonce {
								pool.pendingState.SetNonce(offenders[i], nonce)
							}
							logger.Warn().Str("hash", hash.Hex()).Msg("Removed fairness-exceeding pending transaction")
						}
						pending--
					}
				}
			}
		}
		// If still above threshold, reduce to limit or min allowance
		if pending > pool.config.GlobalSlots && len(offenders) > 0 {
			for pending > pool.config.GlobalSlots && uint64(pool.pending[offenders[len(offenders)-1]].Len()) > pool.config.AccountSlots {
				for _, addr := range offenders {
					list := pool.pending[addr]
					for _, tx := range list.Cap(list.Len() - 1) {
						// Drop the transaction from the global pools too
						hash := tx.Hash()
						pool.all.Remove(hash)
						pool.priced.Removed()
						pool.txErrorSink.Add(tx, fmt.Errorf("fairness-exceeding pending transaction"))

						// Update the account nonce to the dropped transaction
						if nonce := tx.Nonce(); pool.pendingState.GetNonce(addr) > nonce {
							pool.pendingState.SetNonce(addr, nonce)
						}
						logger.Warn().Str("hash", hash.Hex()).Msg("Removed fairness-exceeding pending transaction")
					}
					pending--
				}
			}
		}
		pendingRateLimitCounter.Inc(int64(pendingBeforeCap - pending))
	}
	// If we've queued more transactions than the hard limit, drop oldest ones
	queued := uint64(0)
	for _, list := range pool.queue {
		queued += uint64(list.Len())
	}
	if queued > pool.config.GlobalQueue {
		// Sort all accounts with queued transactions by heartbeat
		addresses := make(addressesByHeartbeat, 0, len(pool.queue))
		for addr := range pool.queue {
			if !pool.locals.contains(addr) { // don't drop locals
				addresses = append(addresses, addressByHeartbeat{addr, pool.beats[addr]})
			}
		}
		sort.Sort(addresses)

		// Drop transactions until the total is below the limit or only locals remain
		for drop := queued - pool.config.GlobalQueue; drop > 0 && len(addresses) > 0; {
			addr := addresses[len(addresses)-1]
			list := pool.queue[addr.address]

			addresses = addresses[:len(addresses)-1]

			// Drop all transactions if they are less than the overflow
			if size := uint64(list.Len()); size <= drop {
				for _, tx := range list.Flatten() {
					pool.txErrorSink.Add(tx, fmt.Errorf("exceeds global cap for queued transactions"))
					pool.removeTx(tx.Hash(), true)
				}
				drop -= size
				queuedRateLimitCounter.Inc(int64(size))
				continue
			}
			// Otherwise drop only last few transactions
			txs := list.Flatten()
			for i := len(txs) - 1; i >= 0 && drop > 0; i-- {
				pool.txErrorSink.Add(txs[i], fmt.Errorf("exceeds global cap for queued transactions"))
				pool.removeTx(txs[i].Hash(), true)
				drop--
				queuedRateLimitCounter.Inc(1)
			}
		}
	}
}

// demoteUnexecutables removes invalid and processed transactions from the pools
// executable/pending queue and any subsequent transactions that become unexecutable
// are moved back into the future queue.
// 交易池降级函数，主要功能是从pending中移除无效的交易，同时将后续的交易都移动到queue。
func (pool *TxPool) demoteUnexecutables(bn uint64) {
	// Iterate over all accounts and demote any non-executable transactions
	logger := utils.Logger().With().Stack().Logger()

	for addr, list := range pool.pending {
		nonce := pool.currentState.GetNonce(addr)

		// Drop all transactions that are deemed too old (low nonce)
		for _, tx := range list.Forward(nonce) {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.priced.Removed()
			logger.Debug().Str("hash", hash.Hex()).Msg("Removed old pending transaction")
			// Do not report to error sink as old txs are on chain or meaningful error caught elsewhere.
		}
		// Drop all transactions that are too costly (low balance or out of gas), and queue any invalids back for later
		drops, errs, invalids := list.FilterValid(pool, addr, bn)
		for i, tx := range drops {
			hash := tx.Hash()
			pool.all.Remove(hash)
			pool.priced.Removed()
			pendingNofundsCounter.Inc(1)
			pool.txErrorSink.Add(tx, errs[i])
			logger.Warn().Str("hash", hash.Hex()).Err(errs[i]).
				Msg("Removed unexecutable pending transaction")
		}
		for _, tx := range invalids {
			hash := tx.Hash()
			logger.Warn().Str("hash", hash.Hex()).Msg("Demoting pending transaction")
			if _, err := pool.enqueueTx(tx); err != nil {
				pool.txErrorSink.Add(tx, err)
			}
		}
		// If there's a gap in front, alert (should never happen)
		// 如果有间隙，将后面的交易移动到queue列表中
		if list.Len() > 0 && list.txs.Get(nonce) == nil {
			for _, tx := range list.Cap(0) {
				hash := tx.Hash()
				logger.Error().Str("hash", hash.Hex()).Msg("Demoting invalidated transaction")
				if _, err := pool.enqueueTx(tx); err != nil {
					pool.txErrorSink.Add(tx, err)
				}
			}
		}
		// Delete the entire queue entry if it became empty.
		// 如果经过上面的降级，pending里某个addr一个交易都没有，就把该账户给删除
		if list.Empty() {
			delete(pool.pending, addr)
			delete(pool.beats, addr)
		}
	}
}

// addressByHeartbeat is an account address tagged with its last activity timestamp.
type addressByHeartbeat struct {
	address   common.Address
	heartbeat time.Time
}

type addressesByHeartbeat []addressByHeartbeat

func (a addressesByHeartbeat) Len() int           { return len(a) }
func (a addressesByHeartbeat) Less(i, j int) bool { return a[i].heartbeat.Before(a[j].heartbeat) }
func (a addressesByHeartbeat) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }

// accountSet is simply a set of addresses to check for existence, and a signer
// capable of deriving addresses from transactions.
type accountSet struct {
	accounts map[common.Address]struct{}
	signer   types.Signer
	cache    *[]common.Address
}

// newAccountSet creates a new address set with the associated signer.
// Note that tx pool will never see an unprotected tx, therefore can use only EIP155 signer.
func newAccountSet(chainID *big.Int) *accountSet {
	return &accountSet{
		accounts: make(map[common.Address]struct{}),
		signer:   types.NewEIP155Signer(chainID),
	}
}

// contains checks if a given address is contained within the set.
func (as *accountSet) contains(addr common.Address) bool {
	_, exist := as.accounts[addr]
	return exist
}

// containsTx checks if the sender of a given tx is within the set. If the sender
// cannot be derived, this method returns false.
func (as *accountSet) containsTx(tx types.PoolTransaction) bool {
	if addr, err := tx.SenderAddress(); err == nil {
		return as.contains(addr)
	}
	return false
}

// add inserts a new address into the set to track.
func (as *accountSet) add(addr common.Address) {
	as.accounts[addr] = struct{}{}
	as.cache = nil
}

// flatten returns the list of addresses within this set, also caching it for later
// reuse. The returned slice should not be changed!
func (as *accountSet) flatten() []common.Address {
	if as.cache == nil {
		accounts := make([]common.Address, 0, len(as.accounts))
		for account := range as.accounts {
			accounts = append(accounts, account)
		}
		as.cache = &accounts
	}
	return *as.cache
}

// txLookup is used internally by TxPool to track transactions while allowing lookup without
// mutex contention.
//
// Note, although this type is properly protected against concurrent access, it
// is **not** a type that should ever be mutated or even exposed outside of the
// transaction pool, since its internal state is tightly coupled with the pools
// internal mechanisms. The sole purpose of the type is to permit out-of-bound
// peeking into the pool in TxPool.Get without having to acquire the widely scoped
// TxPool.mu mutex.
type txLookup struct {
	all  map[common.Hash]types.PoolTransaction
	lock sync.RWMutex
}

// newTxLookup returns a new txLookup structure.
func newTxLookup() *txLookup {
	return &txLookup{
		all: make(map[common.Hash]types.PoolTransaction),
	}
}

// Range calls f on each key and value present in the map.
func (t *txLookup) Range(f func(hash common.Hash, tx types.PoolTransaction) bool) {
	t.lock.RLock()
	defer t.lock.RUnlock()

	for key, value := range t.all {
		if !f(key, value) {
			break
		}
	}
}

// Get returns a transaction if it exists in the lookup, or nil if not found.
func (t *txLookup) Get(hash common.Hash) types.PoolTransaction {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return t.all[hash]
}

// Count returns the current number of items in the lookup.
func (t *txLookup) Count() int {
	t.lock.RLock()
	defer t.lock.RUnlock()

	return len(t.all)
}

// Add adds a transaction to the lookup.
func (t *txLookup) Add(tx types.PoolTransaction) {
	t.lock.Lock()
	defer t.lock.Unlock()

	t.all[tx.Hash()] = tx
}

// Remove removes a transaction from the lookup.
func (t *txLookup) Remove(hash common.Hash) {
	t.lock.Lock()
	defer t.lock.Unlock()

	delete(t.all, hash)
}
