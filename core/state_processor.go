// Copyright 2015 The go-ethereum Authors
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
	"github.com/harmony-one/harmony/shard/mapping/load"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	atomicState "github.com/harmony-one/harmony/atomic/types"
	"github.com/harmony-one/harmony/block"
	consensus_engine "github.com/harmony-one/harmony/consensus/engine"
	"github.com/harmony-one/harmony/consensus/reward"
	"github.com/harmony-one/harmony/core/state"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/core/vm"
	"github.com/harmony-one/harmony/internal/params"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
	"github.com/harmony-one/harmony/staking/slash"
	staking "github.com/harmony-one/harmony/staking/types"
	"github.com/pkg/errors"
)

// StateProcessor is a basic Processor, which takes care of transitioning
// state from one point to another.
//
// StateProcessor implements Processor.
type StateProcessor struct {
	config *params.ChainConfig     // Chain configuration options
	bc     *BlockChain             // Canonical block chain
	engine consensus_engine.Engine // Consensus engine used for block rewards
}

// NewStateProcessor initialises a new StateProcessor.
func NewStateProcessor(
	config *params.ChainConfig, bc *BlockChain, engine consensus_engine.Engine,
) *StateProcessor {
	return &StateProcessor{
		config: config,
		bc:     bc,
		engine: engine,
	}
}

// Process processes the state changes according to the Ethereum rules by running
// the transaction messages using the statedb and applying any rewards to both
// the processor (coinbase) and any included uncles.
//
// Process returns the receipts and logs accumulated during the process and
// returns the amount of gas that was used in the process. If any of the
// transactions failed to execute due to insufficient gas it will return an error.
// 执行harmony的状态改变，在接收到新区块的时候，执行Process函数，得到的结果可以验证接收到的区块的正确性
func (p *StateProcessor) Process(
	block *types.Block, statedb *state.DB, cfg vm.Config,
) (
	types.Receipts, types.CXReceipts,
	[]*types.Log, uint64, reward.Reader, error,
) {
	var (
		receipts types.Receipts
		outcxs   types.CXReceipts
		incxs    = block.IncomingReceipts()
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
	// 从coinbase地址获取相关的ecdsa地址,通常所有地址都ecdsa生成的，所以正常情况就是返回coinbase地址
	beneficiary, err := p.bc.GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}

	// Iterate over and process the individual transactions
	// 迭代处理Transaction事务
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, cxReceipt, _, err := ApplyTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, 0, nil, err
		}
		receipts = append(receipts, receipt)
		if cxReceipt != nil {
			outcxs = append(outcxs, cxReceipt)
		}
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Iterate over and process the staking transactions
	// 处理stakingTransaction事务
	L := len(block.Transactions())
	for i, tx := range block.StakingTransactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i+L)
		receipt, _, err := ApplyStakingTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, 0, nil, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	// incomingReceipts should always be processed
	// after transactions (to be consistent with the block proposal)
	// 处理跨链交易的incomingreceipt
	for _, cx := range block.IncomingReceipts() {
		if err := ApplyIncomingReceipt(
			p.config, statedb, header, cx,
		); err != nil {
			return nil, nil,
				nil, 0, nil, errors.New("[Process] Cannot apply incoming receipts")
		}
	}

	slashes := slash.Records{}
	if s := header.Slashes(); len(s) > 0 {
		if err := rlp.DecodeBytes(s, &slashes); err != nil {
			return nil, nil, nil, 0, nil, errors.New(
				"[Process] Cannot finalize block",
			)
		}
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	sigsReady := make(chan bool)
	go func() {
		// Block processing don't need to block on reward computation as in block proposal
		sigsReady <- true
	}()
	_, payout, err := p.engine.Finalize(
		p.bc, header, statedb, block.Transactions(),
		receipts, outcxs, incxs, block.StakingTransactions(), slashes, sigsReady, func() uint64 { return header.ViewID().Uint64() },
	)
	if err != nil {
		return nil, nil, nil, 0, nil, errors.New("[Process] Cannot finalize block")
	}

	return receipts, outcxs, allLogs, *usedGas, payout, nil
}

/*
	dynamic sharding
 */
// 新增Process1接口，能够处理loadmap
func (p *StateProcessor) Process1(
	block *types.Block, statedb *state.DB, cfg vm.Config, loadmapdb *load.LoadMapDB,
) (
	types.Receipts, types.CXReceipts,
	[]*types.Log, uint64, reward.Reader, error,
) {
	var (
		receipts types.Receipts
		outcxs   types.CXReceipts
		incxs    = block.IncomingReceipts()
		usedGas  = new(uint64)
		header   = block.Header()
		allLogs  []*types.Log
		gp       = new(GasPool).AddGas(block.GasLimit())
	)
	// 从coinbase地址获取相关的ecdsa地址,通常所有地址都ecdsa生成的，所以正常情况就是返回coinbase地址
	beneficiary, err := p.bc.GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, nil, nil, 0, nil, err
	}

	// Iterate over and process the individual transactions
	// 迭代处理Transaction事务
	for i, tx := range block.Transactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i)
		receipt, cxReceipt, _, err := ApplyTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, 0, nil, err
		}
		receipts = append(receipts, receipt)
		if cxReceipt != nil {
			outcxs = append(outcxs, cxReceipt)
		}
		allLogs = append(allLogs, receipt.Logs...)
	}
	// Iterate over and process the staking transactions
	// 处理stakingTransaction事务
	L := len(block.Transactions())
	for i, tx := range block.StakingTransactions() {
		statedb.Prepare(tx.Hash(), block.Hash(), i+L)
		receipt, _, err := ApplyStakingTransaction(
			p.config, p.bc, &beneficiary, gp, statedb, header, tx, usedGas, cfg,
		)
		if err != nil {
			return nil, nil, nil, 0, nil, err
		}
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
	}

	/* dyanmic sharding */
	LStake := len(block.StakingTransactions())
	//fmt.Println("====Receive block sttx====: ", block.StateTransferTransactions())
	// 执行stateTransfer交易
	for i, ss := range block.StateTransferTransactions(){
		// 重要！！准备一个statedb的对象，但是总感觉验证会有点问题哎= =，到底有啥用呢？
		// statedb就存了一个thash，bhash，index，用于设置最新的state？感觉loadmapdb也需要一个
		statedb.Prepare(ss.Hash(), block.Hash(), i+L+LStake)
		receipt, err := ApplyStateTransferTransaction(
			p.config, statedb, header, loadmapdb, ss,
			)
		if err != nil {
			return nil, nil, nil, 0, nil, errors.New("[Process] Cannot apply state transfer transactions")
		}
		// 记录交易的receipt和log
		receipts = append(receipts, receipt)
		allLogs = append(allLogs, receipt.Logs...)
		//for _, receiptItem := range *receipt{
		//	receipts = append(receipts, receiptItem)
		//	allLogs = append(allLogs, receiptItem.Logs...)
		//}
	}
	// END

	// incomingReceipts should always be processed
	// after transactions (to be consistent with the block proposal)
	// 处理跨链交易的incomingreceipt
	for _, cx := range block.IncomingReceipts() {
		if err := ApplyIncomingReceipt(
			p.config, statedb, header, cx,
		); err != nil {
			return nil, nil,
				nil, 0, nil, errors.New("[Process] Cannot apply incoming receipts")
		}
	}

	slashes := slash.Records{}
	if s := header.Slashes(); len(s) > 0 {
		if err := rlp.DecodeBytes(s, &slashes); err != nil {
			return nil, nil, nil, 0, nil, errors.New(
				"[Process] Cannot finalize block",
			)
		}
	}

	// Finalize the block, applying any consensus engine specific extras (e.g. block rewards)
	sigsReady := make(chan bool)
	go func() {
		// Block processing don't need to block on reward computation as in block proposal
		sigsReady <- true
	}()
	// 修改Process1的Finalize函数，新增Finalize1接口
	//_, payout, err := p.engine.Finalize(
	//	p.bc, header, statedb, block.Transactions(),
	//	receipts, outcxs, incxs, block.StakingTransactions(), slashes, sigsReady, func() uint64 { return header.ViewID().Uint64() },
	//)
	_, payout, err := p.engine.Finalize1(
		p.bc, header, statedb, loadmapdb, block.Transactions(),
		receipts, outcxs, incxs, block.StakingTransactions(), block.StateTransferTransactions(), slashes, sigsReady, func() uint64 { return header.ViewID().Uint64() },
	)

	if err != nil {
		return nil, nil, nil, 0, nil, errors.New("[Process] Cannot finalize block")
	}

	return receipts, outcxs, allLogs, *usedGas, payout, nil
}



// return true if it is valid
func getTransactionType(
	config *params.ChainConfig, header *block.Header, tx *types.Transaction,
) types.TransactionType {
	if header.ShardID() == tx.ShardID() &&
		(!config.AcceptsCrossTx(header.Epoch()) ||
			tx.ShardID() == tx.ToShardID()) {
		return types.SameShardTx
	}
	numShards := shard.Schedule.InstanceForEpoch(header.Epoch()).NumShards()
	// Assuming here all the shards are consecutive from 0 to n-1, n is total number of shards
	if tx.ShardID() != tx.ToShardID() &&
		header.ShardID() == tx.ShardID() &&
		tx.ToShardID() < numShards {
		return types.SubtractionOnly
	}
	return types.InvalidTx
}

// ApplyTransaction attempts to apply a transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
// 执行一笔交易，返回收据
func ApplyTransaction(config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.DB, header *block.Header, tx *types.Transaction, usedGas *uint64, cfg vm.Config) (*types.Receipt, *types.CXReceipt, uint64, error) {
	txType := getTransactionType(config, header, tx)
	if txType == types.InvalidTx {
		return nil, nil, 0, errors.New("Invalid Transaction Type")
	}

	if txType != types.SameShardTx && !config.AcceptsCrossTx(header.Epoch()) {
		return nil, nil, 0, errors.Errorf(
			"cannot handle cross-shard transaction until after epoch %v (now %v)",
			config.CrossTxEpoch, header.Epoch(),
		)
	}

	var signer types.Signer
	if tx.IsEthCompatible() {
		if !config.IsEthCompatible(header.Epoch()) {
			return nil, nil, 0, errors.New("ethereum compatible transactions not supported at current epoch")
		}
		signer = types.NewEIP155Signer(config.EthCompatibleChainID)
	} else {
		signer = types.MakeSigner(config, header.Epoch())
	}
	// 将Transaction构建成Message，再传入evm执行
	msg, err := tx.AsMessage(signer)

	// skip signer err for additiononly tx
	if err != nil {
		return nil, nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	// author是coinbase地址
	context := NewEVMContext(msg, header, bc, author)
	context.TxType = txType
	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)
	// Apply the transaction to the current state (included in the env)
	result, err := ApplyMessage(vmenv, msg, gp)
	if err != nil {
		return nil, nil, 0, err
	}
	// Update the state with pending changes
	var root []byte
	if config.IsS3(header.Epoch()) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
	}
	*usedGas += result.UsedGas

	failedExe := result.VMErr != nil
	// Create a new receipt for the transaction, storing the intermediate root and gas used by the tx
	// based on the eip phase, we're passing whether the root touch-delete accounts.
	receipt := types.NewReceipt(root, failedExe, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = result.UsedGas
	// if the transaction created a contract, store the creation address in the receipt.
	if msg.To() == nil {
		receipt.ContractAddress = crypto.CreateAddress(vmenv.Context.Origin, tx.Nonce())
	}

	// Set the receipt logs and create a bloom for filtering
	if config.IsReceiptLog(header.Epoch()) {
		receipt.Logs = statedb.GetLogs(tx.Hash())
	}
	receipt.Bloom = types.CreateBloom(types.Receipts{receipt})

	var cxReceipt *types.CXReceipt
	// Do not create cxReceipt if EVM call failed
	if txType == types.SubtractionOnly && !failedExe {
		cxReceipt = &types.CXReceipt{tx.Hash(), msg.From(), msg.To(), tx.ShardID(), tx.ToShardID(), msg.Value()}
	} else {
		cxReceipt = nil
	}

	return receipt, cxReceipt, result.UsedGas, err
}

// ApplyStakingTransaction attempts to apply a staking transaction to the given state database
// and uses the input parameters for its environment. It returns the receipt
// for the staking transaction, gas used and an error if the transaction failed,
// indicating the block was invalid.
// staking transaction will use the code field in the account to store the staking information
func ApplyStakingTransaction(
	config *params.ChainConfig, bc ChainContext, author *common.Address, gp *GasPool, statedb *state.DB,
	header *block.Header, tx *staking.StakingTransaction, usedGas *uint64, cfg vm.Config) (receipt *types.Receipt, gas uint64, err error) {

	msg, err := StakingToMessage(tx, header.Number())
	if err != nil {
		return nil, 0, err
	}

	// Create a new context to be used in the EVM environment
	context := NewEVMContext(msg, header, bc, author)

	// Create a new environment which holds all relevant information
	// about the transaction and calling mechanisms.
	vmenv := vm.NewEVM(context, statedb, config, cfg)

	// Apply the transaction to the current state (included in the env)
	gas, err = ApplyStakingMessage(vmenv, msg, gp, bc)
	if err != nil {
		return nil, 0, err
	}

	// Update the state with pending changes
	var root []byte
	if config.IsS3(header.Epoch()) {
		statedb.Finalise(true)
	} else {
		root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
	}
	*usedGas += gas
	receipt = types.NewReceipt(root, false, *usedGas)
	receipt.TxHash = tx.Hash()
	receipt.GasUsed = gas

	if config.IsReceiptLog(header.Epoch()) {
		receipt.Logs = statedb.GetLogs(tx.Hash())
		utils.Logger().Info().Interface("CollectReward", receipt.Logs)
	}

	return receipt, gas, nil
}

// ApplyIncomingReceipt will add amount into ToAddress in the receipt
func ApplyIncomingReceipt(
	config *params.ChainConfig, db *state.DB,
	header *block.Header, cxp *types.CXReceiptsProof,
) error {
	if cxp == nil {
		return nil
	}

	for _, cx := range cxp.Receipts {
		if cx == nil || cx.To == nil { // should not happend
			return errors.Errorf(
				"ApplyIncomingReceipts: Invalid incomingReceipt! %v", cx,
			)
		}
		utils.Logger().Info().Interface("receipt", cx).
			Msgf("ApplyIncomingReceipts: ADDING BALANCE %d", cx.Amount)

		if !db.Exist(*cx.To) {
			db.CreateAccount(*cx.To)
		}
		db.AddBalance(*cx.To, cx.Amount)
		db.IntermediateRoot(config.IsS3(header.Epoch())) // 计算root hash值
	}
	return nil
}

// StakingToMessage returns the staking transaction as a core.Message.
// requires a signer to derive the sender.
// put it here to avoid cyclic import
func StakingToMessage(
	tx *staking.StakingTransaction, blockNum *big.Int,
) (types.Message, error) {
	payload, err := tx.RLPEncodeStakeMsg()
	if err != nil {
		return types.Message{}, err
	}
	from, err := tx.SenderAddress()
	if err != nil {
		return types.Message{}, err
	}

	msg := types.NewStakingMessage(from, tx.Nonce(), tx.GasLimit(), tx.GasPrice(), payload, blockNum)
	stkType := tx.StakingType()
	if _, ok := types.StakingTypeMap[stkType]; !ok {
		return types.Message{}, staking.ErrInvalidStakingKind
	}
	msg.SetType(types.StakingTypeMap[stkType])
	return msg, nil
}


/**
	dynamic sharding
 */
// 执行StateTransfer类型的交易
// 执行：freeze address的状态，修改address状态，修改loadMap状态
func ApplyStateTransferTransaction(
	config *params.ChainConfig, statedb *state.DB,
	header *block.Header, loadmapdb *load.LoadMapDB, stx *atomicState.StateTransferTransaction,
	) (receipt *types.Receipt, err error) {
	if stx == nil{
		return receipt, nil
	}
	currentShardID := header.ShardID()
	var root []byte // 初始化receipt的root
	switch currentShardID {
	case stx.SourceShardID():
		// 如果当前链的分片=SourceShardID (chain.shardID == stx.data.SourceShardID), 则:
		// 1. 对address的state进行freeze处理
		// 2. 更新loadmap中该address的状态
		utils.Logger().Info().Interface("stateTransfer", stx).
			Msgf("ApplyStateTransferTransaction: Source Shard execute Address %s", stx.Address().String())

		addr := stx.Address()
		if stx.ToFreeze() { // 对于toFreeze=true的交易，执行状态锁定
			utils.Logger().Info().Interface("stateTransfer", stx).
				Msgf("ApplyStateTransferTransaction(SourceShard): Freeze State of Address %s", stx.Address().String())
			//fmt.Println("[Source Chain execute Propose]")
			if !statedb.Exist(addr) {
				statedb.CreateAccount(addr)
				// 设置state的Nonce为1, Balance为0
				//statedb.SetNonce(addr, uint64(1))
				statedb.SetBalance(addr, big.NewInt(0))
			}
			// freeze 使得地址的nonce+1
			//statedb.SetNonce(addr, statedb.GetNonce(addr)+1)
			statedb.SetFreezeState(addr, true)
			// Update the state with pending changes
			// 更新statedb的状态root (由于不用tx来更新，所以参照receipt的方式，直接更新statedb的root)
			if config.IsS3(header.Epoch()) {
				//fmt.Println("===========Finalize HERE==========")
				// 遍历新区块中更新的账户（journal里标记为的dirty的账户）；
				// 删除已经销毁、空的账户数据；
				// 将更新的账户（dirty账户）加入到 loadMapObjectsPending 和 loadMapObjectsDirty
				statedb.Finalise(true)
			} else {
				root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
			}
			// 修改了state状态，构建statedb的收据receipt, 这些交易不产生gas消耗
			//receipt := types.NewReceipt(root, false, 0)
			//receipt.TxHash = stx.Hash()
			//receipt.GasUsed = 0
		}else{ // 对于非状态锁定的交易，更新loadmap; 在commit阶段之后执行
			utils.Logger().Info().Interface("stateTransfer", stx).
				Msgf("ApplyStateTransferTransaction(SourceShard): Modify LoadMap of Address %s", stx.Address().String())
			//fmt.Println("[Source Chain execute Commit]")
			if !loadmapdb.Exist(addr){
				loadmapdb.CreateMapAccount(addr)
			}
			loadmapdb.SetTargetShardID(addr, stx.TargetShardID())
			// 更新loadmapdb的状态root
			//var root []byte
			if config.IsS3(header.Epoch()) {
				loadmapdb.Finalise(true)
			} else {
				root = loadmapdb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
			}
			// 修改了loadMap，构建loadmapdb的收据receipt, 这些交易不产生gas消耗
			//receipt := types.NewReceipt(root, false, 0)
			//receipt.TxHash = stx.Hash()
			//receipt.GasUsed = 0
			//*receipts = append(*receipts, receipt)
		}

	case stx.TargetShardID():
		// 如若当前链的分片=TargetShardID, 则:
		// 1. 更新address的state
		// 2. 更新loadmap中该address的状态
		// 在target shard中，Tofreeze=true的迁移交易不会从freezeCache中读取出来执行，会直接移动到commitPool.cached中
		utils.Logger().Info().Interface("stateTransfer", stx).
			Msgf("ApplyStateTransferTransaction: Target Shard execute Address %s", stx.Address().String())
		// 如果是commitPool.freezeCache来的消息，不做状态的改变直接返回
		if stx.ToFreeze(){
			//fmt.Println("[Target Chain execute Propose]")
			//var root []byte
			if config.IsS3(header.Epoch()) {
				statedb.Finalise(true)
			} else {
				root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
			}
			// 该交易不做任何state状态的修改，构建statedb的收据receipt, 这些交易不产生gas消耗
		} else {
			// 如果是commitPool.committed来的消息
			// 在完成commit操作之后，系统从commitPool的Committed中提取消息创建交易
			// 在target shard中将同时更新statedb和loadmapdb
			// 更新statedb
			//fmt.Println("[Target Chain execute Commit]")
			addr := stx.Address()
			if !statedb.Exist(addr) {
				statedb.CreateAccount(addr)
			}
			statedb.SetFreezeState(addr, false) // 解锁账户
			statedb.SetBalance(addr, stx.Balance()) // 目前只更新balance做转账操作
			statedb.SetNonce(addr, stx.Nonce()) // 设置账户的Nonce
			// TODO: 增加迁移智能合约的操作
			// 更新loadmapdb
			if !loadmapdb.Exist(addr){
				loadmapdb.CreateMapAccount(addr)
			}
			loadmapdb.SetTargetShardID(addr, stx.TargetShardID()) // 设置账户的目标分片ID

			// 更新root，创建receipt
			// [注意！！] 这里只计算修改statedb产生的receipt，不计算修改loadMapdb的receipt
			//var root1, root2 []byte
			//var root[]byte
			if config.IsS3(header.Epoch()) {
				statedb.Finalise(true)
				loadmapdb.Finalise(true)
			} else {
				root = statedb.IntermediateRoot(config.IsS3(header.Epoch())).Bytes()
			}
			//statedb.IntermediateRoot(config.IsS3(header.Epoch()))
			//receipt := types.NewReceipt(root, false, 0)
			//receipt.TxHash = stx.Hash()
			//receipt.GasUsed = 0
		}

	}
	// 在switch之外创建receipt
	receipt = types.NewReceipt(root, false, 0)
	receipt.TxHash = stx.Hash()
	receipt.GasUsed = 0

	return receipt, nil
}
