package node

import (
	"errors"
	"fmt"
	atomicState "github.com/harmony-one/harmony/atomic/types"
	"sort"
	"strings"
	"time"

	"github.com/harmony-one/harmony/consensus"

	"github.com/harmony-one/harmony/crypto/bls"

	staking "github.com/harmony-one/harmony/staking/types"

	"github.com/ethereum/go-ethereum/common"
	"github.com/harmony-one/harmony/core/rawdb"
	"github.com/harmony-one/harmony/core/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/harmony-one/harmony/shard"
)

// Constants of proposing a new block
const (
	SleepPeriod           = 20 * time.Millisecond
	IncomingReceiptsLimit = 6000 // 2000 * (numShards - 1)
	// 暂定每个区块最多只能处理的迁移交易sttx数量
	maxProcessSttxNum = 299
)

// WaitForConsensusReadyV2 listen for the readiness signal from consensus and generate new block for consensus.
// only leader will receive the ready signal
func (node *Node) WaitForConsensusReadyV2(readySignal chan consensus.ProposalType, commitSigsChan chan []byte, stopChan chan struct{}, stoppedChan chan struct{}) {
	go func() {
		// Setup stoppedChan
		defer close(stoppedChan)

		utils.Logger().Debug().
			Msg("Waiting for Consensus ready")
		select {
		case <-time.After(30 * time.Second):
		case <-stopChan:
			return
		}

		for {
			// keep waiting for Consensus ready
			select {
			case <-stopChan:
				utils.Logger().Warn().
					Msg("Consensus new block proposal: STOPPED!")
				return
			case proposalType := <-readySignal:
				retryCount := 0
				// 只有leader能打包区块
				for node.Consensus != nil && node.Consensus.IsLeader() {
					time.Sleep(SleepPeriod)
					utils.Logger().Info().
						Uint64("blockNum", node.Blockchain().CurrentBlock().NumberU64()+1).
						Bool("asyncProposal", proposalType == consensus.AsyncProposal).
						Msg("PROPOSING NEW BLOCK ------------------------------------------------")

					// Prepare last commit signatures
					newCommitSigsChan := make(chan []byte)

					go func() {
						waitTime := 0 * time.Second
						if proposalType == consensus.AsyncProposal {
							waitTime = consensus.CommitSigReceiverTimeout
						}
						select {
						case <-time.After(waitTime): // 超时处理
							if waitTime == 0 {
								utils.Logger().Info().Msg("[ProposeNewBlock] Sync block proposal, reading commit sigs directly from DB")
							} else {
								utils.Logger().Info().Msg("[ProposeNewBlock] Timeout waiting for commit sigs, reading directly from DB")
							}
							sigs, err := node.Consensus.BlockCommitSigs(node.Blockchain().CurrentBlock().NumberU64())

							if err != nil {
								utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Cannot get commit signatures from last block")
							} else {
								newCommitSigsChan <- sigs
							}
						case commitSigs := <-commitSigsChan: // 接收到签名信号的处理
							utils.Logger().Info().Msg("[ProposeNewBlock] received commit sigs asynchronously")
							if len(commitSigs) > bls.BLSSignatureSizeInBytes {
								utils.Logger().Info().
									Int("receiveNumber", len(commitSigs)).
									Msg("[ProposeNewBlock] received commit sigs asynchronously enough")
								newCommitSigsChan <- commitSigs
							}
						}
					}()
					newBlock, err := node.ProposeNewBlock(newCommitSigsChan)
					//fmt.Println("=======NEW BLOCK====== ", newBlock.StateTransferTransactions())
					fmt.Println("=======NEW BLOCK====== txNum: ",len(newBlock.Transactions()), "sttxNum: ", len(newBlock.StateTransferTransactions()))

					// 提案区块错误处理
					if err == nil {
						if blk, ok := node.proposedBlock[newBlock.NumberU64()]; ok {
							utils.Logger().Info().Uint64("blockNum", newBlock.NumberU64()).Str("blockHash", blk.Hash().Hex()).
								Msg("Block with the same number was already proposed, abort.")
							break
						}
						utils.Logger().Info().
							Uint64("blockNum", newBlock.NumberU64()).
							Uint64("epoch", newBlock.Epoch().Uint64()).
							Uint64("viewID", newBlock.Header().ViewID().Uint64()).
							Int("numTxs", newBlock.Transactions().Len()).
							Int("numStakingTxs", newBlock.StakingTransactions().Len()).
							Int("crossShardReceipts", newBlock.IncomingReceipts().Len()).
							Msg("=========Successfully Proposed New Block==========")

						// Send the new block to Consensus so it can be confirmed.
						node.proposedBlock[newBlock.NumberU64()] = newBlock
						delete(node.proposedBlock, newBlock.NumberU64()-10)
						node.BlockChannel <- newBlock
						break
					} else {
						retryCount++
						utils.Logger().Err(err).Int("retryCount", retryCount).
							Msg("!!!!!!!!!Failed Proposing New Block!!!!!!!!!")
						if retryCount > 3 {
							// break to avoid repeated failures
							break
						}
						continue
					}
				}
			}
		}
	}()
}

// ProposeNewBlock proposes a new block...
// 提案一个新区块流程
func (node *Node) ProposeNewBlock(commitSigs chan []byte) (*types.Block, error) {
	// 先获取当前header，epoch，blockHeight
	currentHeader := node.Blockchain().CurrentHeader()
	nowEpoch, blockNow := currentHeader.Epoch(), currentHeader.Number()
	utils.AnalysisStart("ProposeNewBlock", nowEpoch, blockNow)
	defer utils.AnalysisEnd("ProposeNewBlock", nowEpoch, blockNow)

	node.Worker.UpdateCurrent()

	header := node.Worker.GetCurrentHeader()
	// Update worker's current header and
	// state data in preparation to propose/process new transactions
	leaderKey := node.Consensus.LeaderPubKey
	var (
		coinbase    = node.GetAddressForBLSKey(leaderKey.Object, header.Epoch())
		beneficiary = coinbase
		err         error
	)

	// After staking, all coinbase will be the address of bls pub key
	if node.Blockchain().Config().IsStaking(header.Epoch()) {
		blsPubKeyBytes := leaderKey.Object.GetAddress()
		coinbase.SetBytes(blsPubKeyBytes[:])
	}

	emptyAddr := common.Address{}
	if coinbase == emptyAddr {
		return nil, errors.New("[ProposeNewBlock] Failed setting coinbase")
	}

	// Must set coinbase here because the operations below depend on it
	header.SetCoinbase(coinbase)

	// Get beneficiary based on coinbase
	// Before staking, coinbase itself is the beneficial
	// After staking, beneficial is the corresponding ECDSA address of the bls key
	beneficiary, err = node.Blockchain().GetECDSAFromCoinbase(header)
	if err != nil {
		return nil, err
	}

	// Add VRF
	// 创建区块的时候，会为当前区块生成一个新的VRF
	if node.Blockchain().Config().IsVRF(header.Epoch()) {
		//generate a new VRF for the current block
		if err := node.Consensus.GenerateVrfAndProof(header); err != nil {
			return nil, err
		}
	}

	// Prepare normal and staking transactions retrieved from transaction pool
	// 利用从pool中获取的交易，准备普通交易和staking交易
	utils.AnalysisStart("proposeNewBlockChooseFromTxnPool")

	// 从交易池里取交易
	pendingPoolTxs, err := node.TxPool.Pending()
	if err != nil {
		utils.Logger().Err(err).Msg("Failed to fetch pending transactions")
		return nil, err
	}
	pendingPlainTxs := map[common.Address]types.Transactions{}
	pendingStakingTxs := staking.StakingTransactions{}
	for addr, poolTxs := range pendingPoolTxs {
		plainTxsPerAcc := types.Transactions{}
		for _, tx := range poolTxs {
			if plainTx, ok := tx.(*types.Transaction); ok {
				//fmt.Println("Normal TX size: ",plainTx.Size()) // dynamic sharding
				plainTxsPerAcc = append(plainTxsPerAcc, plainTx)
			} else if stakingTx, ok := tx.(*staking.StakingTransaction); ok {
				// Only process staking transactions after pre-staking epoch happened.
				if node.Blockchain().Config().IsPreStaking(node.Worker.GetCurrentHeader().Epoch()) {
					pendingStakingTxs = append(pendingStakingTxs, stakingTx)
				}
			} else {
				utils.Logger().Err(types.ErrUnknownPoolTxType).
					Msg("Failed to parse pending transactions")
				return nil, types.ErrUnknownPoolTxType
			}
		}
		if plainTxsPerAcc.Len() > 0 {
			pendingPlainTxs[addr] = plainTxsPerAcc
		}
	}
	utils.AnalysisEnd("proposeNewBlockChooseFromTxnPool")

	// Try commit normal and staking transactions based on the current state
	// The successfully committed transactions will be put in the proposed block
	/*
	if err := node.Worker.CommitTransactions(
		pendingPlainTxs, pendingStakingTxs, beneficiary,
	); err != nil {
		utils.Logger().Error().Err(err).Msg("cannot commit transactions")
		return nil, err
	}
	*/
	/* dynamic sharding*/
	// 打包区块时，从freezeCache和committed中获取全部的stateTransfer交易
	freezeCachePoolTxs, err := node.CommitPool.FreezeCache()
	if err != nil {
		utils.Logger().Err(err).Msg("Failed to fetch freezeCache sttx transactions")
		return nil, err
	}
	committedPoolTxs, err := node.CommitPool.Committed()
	if err != nil {
		utils.Logger().Err(err).Msg("Failed to fetch committed sttx transactions")
		return nil, err
	}
	pendingStateTransferTxs := atomicState.StateTransferTransactions{}
	//cumulateSttxNum := 0
	for _, poolTxs := range freezeCachePoolTxs{
		//fmt.Println("=======NODE NEWBLOCK freezeCachePoolTxs====", freezeCachePoolTxs)
		//fmt.Println("Tx size: ", tx.Size())
		// 限制每个区块处理的sttx总数(最低数量)
		//if len(freezeCachePoolTxs) < maxProcessSttxNum{
		//	break
		//}
		// for range直接取值的地址是找不到的
		tx := poolTxs
		pendingStateTransferTxs = append(pendingStateTransferTxs, &tx)
	}
	for _, poolTxs := range committedPoolTxs{
		//fmt.Println("=======NODE NEWBLOCK committedPoolTxs====", committedPoolTxs)
		//if len(committedPoolTxs) < maxProcessSttxNum{
		//	break
		//}
		tx := poolTxs
		pendingStateTransferTxs = append(pendingStateTransferTxs, &tx)
	}

	// 提交normal, staking, stateTransfer交易based on the current state【完成】
	if err := node.Worker.CommitTransactions1(
		pendingPlainTxs, pendingStakingTxs, pendingStateTransferTxs, beneficiary,
	); err != nil {
		utils.Logger().Error().Err(err).Msg("cannot commit transactions")
		return nil, err
	}
	// END

	// Prepare cross shard transaction receipts
	// 获取跨片交易的收据的proof, 最后执行CommitReceipt
	receiptsList := node.proposeReceiptsProof()
	if len(receiptsList) != 0 {
		if err := node.Worker.CommitReceipts(receiptsList); err != nil {
			return nil, err
		}
	}

	isBeaconchainInCrossLinkEra := node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsCrossLink(node.Worker.GetCurrentHeader().Epoch())

	isBeaconchainInStakingEra := node.NodeConfig.ShardID == shard.BeaconChainShardID &&
		node.Blockchain().Config().IsStaking(node.Worker.GetCurrentHeader().Epoch())

	utils.AnalysisStart("proposeNewBlockVerifyCrossLinks")
	// Prepare cross links and slashing messages (测试是false)
	var crossLinksToPropose types.CrossLinks
	if isBeaconchainInCrossLinkEra {
		allPending, err := node.Blockchain().ReadPendingCrossLinks()
		invalidToDelete := []types.CrossLink{}
		if err == nil {
			for _, pending := range allPending {
				exist, err := node.Blockchain().ReadCrossLink(pending.ShardID(), pending.BlockNum())
				if err == nil || exist != nil {
					invalidToDelete = append(invalidToDelete, pending)
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] pending crosslink is already committed onchain", err)
					continue
				}

				// Crosslink is already verified before it's accepted to pending,
				// no need to verify again in proposal.
				if !node.Blockchain().Config().IsCrossLink(pending.Epoch()) {
					utils.Logger().Debug().
						AnErr("[ProposeNewBlock] pending crosslink that's before crosslink epoch", err)
					continue
				}

				crossLinksToPropose = append(crossLinksToPropose, pending)
				if len(crossLinksToPropose) > 15 {
					break
				}
			}
			utils.Logger().Info().
				Msgf("[ProposeNewBlock] Proposed %d crosslinks from %d pending crosslinks",
					len(crossLinksToPropose), len(allPending),
				)
		} else {
			utils.Logger().Error().Err(err).Msgf(
				"[ProposeNewBlock] Unable to Read PendingCrossLinks, number of crosslinks: %d",
				len(allPending),
			)
		}
		node.Blockchain().DeleteFromPendingCrossLinks(invalidToDelete)
	}
	utils.AnalysisEnd("proposeNewBlockVerifyCrossLinks")

	if isBeaconchainInStakingEra { // 测试不考虑staking
		// this will set a meaningful w.current.slashes
		if err := node.Worker.CollectVerifiedSlashes(); err != nil {
			return nil, err
		}
	}

	// Prepare shard state
	var shardState *shard.State
	if shardState, err = node.Blockchain().SuperCommitteeForNextEpoch(
		node.Beaconchain(), node.Worker.GetCurrentHeader(), false,
	); err != nil {
		return nil, err
	}

	viewIDFunc := func() uint64 {
		return node.Consensus.GetCurBlockViewID()
	}
	// 这里的FinalizeNewBlock是干嘛的？
	// FinalizeNewBlock为下一轮共识生成一个新块 (真正构建区块的地方？)
	// 返回一个block
	finalizedBlock, err := node.Worker.FinalizeNewBlock(
		commitSigs, viewIDFunc,
		coinbase, crossLinksToPropose, shardState,
	)
	if err != nil {
		//fmt.Println("[ProposeNewBlock] Failed finalizing the new block")
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed finalizing the new block")
		return nil, err
	}
	// 验证新区块头(主要是调用/internal/chain/engine.go的VerifyHeader函数)
	utils.Logger().Info().Msg("[ProposeNewBlock] verifying the new block header")
	err = node.Blockchain().Validator().ValidateHeader(finalizedBlock, true)

	if err != nil {
		//fmt.Println("[ProposeNewBlock] Failed verifying the new block header")
		utils.Logger().Error().Err(err).Msg("[ProposeNewBlock] Failed verifying the new block header")
		return nil, err
	}
	//fmt.Println("=========ProposeNewBlock END============")
	return finalizedBlock, nil
}

func (node *Node) proposeReceiptsProof() []*types.CXReceiptsProof {
	if !node.Blockchain().Config().HasCrossTxFields(node.Worker.GetCurrentHeader().Epoch()) {
		return []*types.CXReceiptsProof{}
	}

	numProposed := 0
	validReceiptsList := []*types.CXReceiptsProof{}
	pendingReceiptsList := []*types.CXReceiptsProof{}

	node.pendingCXMutex.Lock()
	defer node.pendingCXMutex.Unlock()

	// not necessary to sort the list, but we just prefer to process the list ordered by shard and blocknum
	pendingCXReceipts := []*types.CXReceiptsProof{}
	for _, v := range node.pendingCXReceipts {
		pendingCXReceipts = append(pendingCXReceipts, v)
	}

	sort.SliceStable(pendingCXReceipts, func(i, j int) bool {
		shardCMP := pendingCXReceipts[i].MerkleProof.ShardID < pendingCXReceipts[j].MerkleProof.ShardID
		shardEQ := pendingCXReceipts[i].MerkleProof.ShardID == pendingCXReceipts[j].MerkleProof.ShardID
		blockCMP := pendingCXReceipts[i].MerkleProof.BlockNum.Cmp(
			pendingCXReceipts[j].MerkleProof.BlockNum,
		) == -1
		return shardCMP || (shardEQ && blockCMP)
	})

	m := map[common.Hash]struct{}{}

Loop:
	for _, cxp := range node.pendingCXReceipts {
		if numProposed > IncomingReceiptsLimit {
			pendingReceiptsList = append(pendingReceiptsList, cxp)
			continue
		}
		// check double spent
		if node.Blockchain().IsSpent(cxp) {
			utils.Logger().Debug().Interface("cxp", cxp).Msg("[proposeReceiptsProof] CXReceipt is spent")
			continue
		}
		hash := cxp.MerkleProof.BlockHash
		// ignore duplicated receipts
		if _, ok := m[hash]; ok {
			continue
		} else {
			m[hash] = struct{}{}
		}

		for _, item := range cxp.Receipts {
			if item.ToShardID != node.Blockchain().ShardID() {
				continue Loop
			}
		}

		if err := node.Blockchain().Validator().ValidateCXReceiptsProof(cxp); err != nil {
			if strings.Contains(err.Error(), rawdb.MsgNoShardStateFromDB) {
				pendingReceiptsList = append(pendingReceiptsList, cxp)
			} else {
				utils.Logger().Error().Err(err).Msg("[proposeReceiptsProof] Invalid CXReceiptsProof")
			}
			continue
		}

		utils.Logger().Debug().Interface("cxp", cxp).Msg("[proposeReceiptsProof] CXReceipts Added")
		validReceiptsList = append(validReceiptsList, cxp)
		numProposed = numProposed + len(cxp.Receipts)
	}

	node.pendingCXReceipts = make(map[string]*types.CXReceiptsProof)
	for _, v := range pendingReceiptsList {
		blockNum := v.Header.Number().Uint64()
		shardID := v.Header.ShardID()
		key := utils.GetPendingCXKey(shardID, blockNum)
		node.pendingCXReceipts[key] = v
	}

	utils.Logger().Debug().Msgf("[proposeReceiptsProof] number of validReceipts %d", len(validReceiptsList))
	return validReceiptsList
}
