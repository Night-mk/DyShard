/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/2/21$ 6:54 PM$
 **/
package atomic

import (
	"context"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethdb"
	"github.com/ethereum/go-ethereum/light"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/ethereum/go-ethereum/trie"
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	"github.com/harmony-one/harmony/atomic/types"
	"github.com/harmony-one/harmony/internal/utils"
	"github.com/pkg/errors"
	"time"
)

/**
	dynamic sharding
 */

/*
	Propose消息处理
 */
// follower server处理propose逻辑
func (s *ServerShard) ProposeHandler(ctx context.Context, req *c_pb.ProposeRequest, hook func(req *c_pb.ProposeRequest) bool) (*c_pb.ProposeResponse, error) {
	var (
		response *c_pb.ProposeResponse
	)
	// follower处理propose时初始化type, 和Node在的链的source还是target
	s.FollowerType = c_pb.FollowerType_SOURCE
	if req.Target == s.CommitPool.chain.ShardID() {
		s.FollowerType = c_pb.FollowerType_TARGET
	}

	// hook函数检查请求是否正确（目前都直接返回true）
	if hook(req) {
		/**
			重要逻辑： 同步执行分片之间的数据传输协议，只有：数据传输完毕+验证 后才能return
		*/
		utils.Logger().Info().Msgf("[ProposeHandler] propose received tx Index=[%d]", req.Index)

		// 初始化signal传入 (应该先初始化?)
		//s.CommitPool.InitSignal(req.Index)
		// follower server节点处理stateTransferMsg消息
		// 1. 验证交易
		// 2. 写入CommitPool的freezeCache缓存
		err := s.HandleStateTransferMsg(req.Index, req.Source, req.Target, req.MsgPayload); if err!=nil{
			utils.Logger().Info().Msg("HandleStateTransferMsg error")
		}

		// follower server节点设置cache状态数据[state data + proof]
		response = &c_pb.ProposeResponse{
			Acktype: c_pb.AckType_ACK, Index: req.Index, Ftype: s.FollowerType,
			Address: req.MsgPayload, MsgPayload: []byte{},
		}
		/* source & target 不同的处理 */
		// source follower锁定state
		if s.FollowerType == c_pb.FollowerType_SOURCE{
			utils.Logger().Info().Msgf("Source freeze the state of tx: %d", req.Index)
			// 等待区块链进程通知propose处理完成
			//fmt.Println("===========Propose Source==========")
			//fmt.Println("[ProposeHandler] Source freeze")
			Loop:
				for {
					select {
						case sig := <- s.CommitPool.Signal[req.Index]:
							// 接收到事件处理完成的消息，就顺序执行后续操作
							if sig == 1{ // propose阶段写入1表示处理成功
								//fmt.Println("[ProposeHandler] Source freeze success")
								utils.Logger().Info().Msg("[ProposeHandler] Source freeze success")
								/* ============Freeze 后处理============= */
								// 1. 获取已经freeze的状态balance, nonce [在这里存在同时读写的情况？]
								// 改为使用pendingState读取当前状态，pendingState是当前最新
								//fmt.Println("[ProposeHandler] read state")
								//if !s.CommitPool.GetPendingState().Exist(common.BytesToAddress(req.MsgPayload)){// 都是打印到这里 no this state
								//	fmt.Println("[ProposeHandler] no this state", common.BytesToAddress(req.MsgPayload).String())
								//}else{
								//	fmt.Println("[ProposeHandler] has this state", common.BytesToAddress(req.MsgPayload).String())
								//}
								balance := s.CommitPool.GetPendingState().GetBalance(common.BytesToAddress(req.MsgPayload))
								nonce := s.CommitPool.GetPendingState().GetNonce(common.BytesToAddress(req.MsgPayload))
								// 2. 构建执行rlp编码，放入proposeResponse
								stateData := types.StateData{}
								stateData.State.Address = common.BytesToAddress(req.MsgPayload)
								stateData.State.Balance = balance
								stateData.State.Nonce = nonce
								stateData.State.Data = []byte{}
								rlpState, _ := rlp.EncodeToBytes(stateData)

								response = &c_pb.ProposeResponse{Acktype: c_pb.AckType_ACK, Index: req.Index, Ftype: s.FollowerType, Address: req.MsgPayload, MsgPayload: rlpState}
								break Loop
							}else{
								s.CommitPool.CloseSignal(req.Index)
								fmt.Println("[ProposeHandler] Source freeze another, Failed index: ", req.Index)
								response = &c_pb.ProposeResponse{Acktype: c_pb.AckType_NACK, Index: req.Index, Ftype: s.FollowerType, Address: req.MsgPayload, MsgPayload: []byte{}}
								break Loop
							}

						case <-time.After(time.Duration(10)*time.Minute): // 超时，事件失败
							s.CommitPool.CloseSignal(req.Index)
							utils.Logger().Error().Msg("[ProposeHandler] Source freeze failure")
							response = &c_pb.ProposeResponse{Acktype: c_pb.AckType_NACK, Index: req.Index, Ftype: s.FollowerType, Address: req.MsgPayload, MsgPayload: []byte{}}
							break Loop
					}
				}
		}else{
			// target follower 也要等待消息上链，确定后直接返回success
			utils.Logger().Info().Msg("[ProposeHandler] go to Target")
			Loop1:
				for {
					select {
					case sig := <- s.CommitPool.Signal[req.Index]:
						// 接收到事件处理完成的消息，就顺序执行后续操作
						if sig == 1{ // propose阶段写入1表示处理成功
							//fmt.Println("[ProposeHandler] Target success")
							utils.Logger().Info().Msg("[ProposeHandler] Target success")
							break Loop1
						}
					case <-time.After(time.Duration(10)*time.Minute): // 超时，事件失败
						s.CommitPool.CloseSignal(req.Index)
						utils.Logger().Error().Msg("[ProposeHandler] Target failure")
						response = &c_pb.ProposeResponse{Acktype: c_pb.AckType_NACK, Index: req.Index, Ftype: s.FollowerType, Address: req.MsgPayload, MsgPayload: []byte{}}
						break Loop1
					}
				}
			utils.Logger().Info().Msg("[ProposeHandler] verify success")
		}

	} else {
		// 不满足hook请求时，返回NACK
		response = &c_pb.ProposeResponse{Acktype: c_pb.AckType_NACK, Index: req.Index, Ftype: s.FollowerType, Address: req.MsgPayload, MsgPayload: []byte{}}
	}
	utils.Logger().Info().
		Str("nodeAddress",s.Config.Nodeaddr).
		Uint64("index", req.Index).
		Msg("===============PROPOSE END==============")
	//fmt.Println("===============PROPOSE END==============", s.Config.Nodeaddr, "index=[", req.Index,"]")
	return response, nil
}


// follower server 处理commit逻辑
func (s *ServerShard) CommitHandler(ctx context.Context, req *c_pb.CommitRequest, hook func(req *c_pb.CommitRequest) bool) (*c_pb.Response, error) {
	var (
		response *c_pb.Response
	)
	reqAddr := common.BytesToAddress(req.Address)
	//fmt.Println("===============COMMIT START==============", s.Config.Nodeaddr, "index=[", req.Index,"]")
	if hook(req) {
		//fmt.Printf("[CommitHandler] %s Committing tx Index: %d\n", s.Addr, req.Index)
		// 查看commitPool.cached中是否有request相关的数据
		_, ok := s.CommitPool.CachedById(req.Index)
		if !ok {
			s.CommitPool.deleteTxFromCached(req.Index)
			utils.Logger().Error().Msgf("[CommitHandler] TX index: [%d],  Error: no value in node cache",req.Index)
			return &c_pb.Response{
				Acktype:  c_pb.AckType_NACK}, errors.New(fmt.Sprintf("no value in node cache on the index %d", req.Index))
		}
		/**
			source & target 有相同的处理，都是删除CommitPool的cached，增加committed
		*/
		s.HandleCommitMsg(req.Source, req.Target, reqAddr, req.Index, req.MsgPayload)
		//fmt.Println("===============COMMIT HandleCommitMsg END==============")

		// 等待将状态数据插入区块链完成的通知
		response = &c_pb.Response{Acktype: c_pb.AckType_ACK, Index: req.Index, Ftype: s.FollowerType}
		Loop:
			for {
				select {
					case sig := <- s.CommitPool.Signal[req.Index]:
						// 接收到事件处理完成的消息，就顺序执行后续操作
						if sig == 2{ // commit阶段写入2表示处理成功
							//if s.FollowerType == c_pb.FollowerType_SOURCE{
							//	fmt.Println("[CommitHandler] Write blockchain success Source")
							//}else{
							//	fmt.Println("[CommitHandler] Write blockchain success Target")
							//}

							s.CommitPool.CloseSignal(req.Index)
							utils.Logger().Info().Msg("[CommitHandler] Write blockchain success")
							break Loop
						}
					case <-time.After(time.Duration(10)*time.Minute): // 超时，事件失败
						s.CommitPool.CloseSignal(req.Index)
						utils.Logger().Error().Msg("[CommitHandler] write blockchain failure")
						response = &c_pb.Response{Acktype: c_pb.AckType_ACK, Index: req.Index, Ftype: s.FollowerType}
						break Loop
				}
			}
		// commit之后，将交易从committed删除
		//s.CommitPool.deleteTxFromCommitted(reqAddr, req.Index)
	} else {
		//s.CommitPool.deleteTxFromCommitted(reqAddr, req.Index)
		response = &c_pb.Response{Acktype: c_pb.AckType_NACK, Index: req.Index, Ftype: s.FollowerType}
	}

	fmt.Println("===============COMMIT END==============", s.Config.Nodeaddr, "index=[", req.Index,"]")
	return response, nil
}


// 处理消息StateTransferMsg
// 这里的StateTransferMsg消息来自2PC的rpc调用
func (s *ServerShard)HandleStateTransferMsg(index uint64, source uint32, target uint32, msgPayload []byte) error{
	// 1. rlp解码 propose的payload
	//stateTransferMsg := types.StateTransferMsg{}
	//err := rlp.DecodeBytes(msgPayload, &stateTransferMsg)
	//if err != nil{
	//	utils.Logger().Error().Err(err).Msg("stateTransfer msg decode failure")
	//	return
	//}
	// propose的payload就是交易的address(不用解码)
	address := common.BytesToAddress(msgPayload)

	// 2. 验证stateTransfer交易 (暂时删除)
	//err = s.validateStateTransferMsg(stateTransferMsg.MsgPayload)
	//if err != nil{
	//	utils.Logger().Error().Err(err).Msg("stateTransfer msg validate failure")
	//	return
	//}
	// 2. end
	var err error
	// 处理StateTransferMsg
	// 1. 判断cached中是否已经有指定index的交易
	_, ok := s.CommitPool.cached.Get(index)
	if !ok{
		// 如果cached中没有该index的交易，则说明这个交易第一次写入；
		// 否则说明共识的时候已经将交易写入了cached，不用重复将交易写入freezeCache
		// 将State添加到commitPool freezeCache
		err = s.CommitPool.addFreezeTransaction(index, source, target, address)
		return err
	}
	return nil
}

func (s *ServerShard)HandleCommitMsg(source uint32, target uint32, addr common.Address, index uint64, MsgPayload []byte) {
	// 1. 判断该交易是否已经在insertChain的逻辑中提交了
	_, ok := s.CommitPool.GetCommittedTx(index)
	// 如果这里没有提交，则将解析的交易写入committed缓存
	if !ok{
		// 1.1 rlp解码commit request里的MsgPayload
		var deState types.StateData
		err := rlp.DecodeBytes(MsgPayload, &deState)
		if err != nil{
			utils.Logger().Info().Msg("[CommitHandler]: msgPayload decode failure")
		}
		// 1.2 将交易写入committed
		s.CommitPool.addCommittedTransaction(source, target, addr, index, deState)
	}
	// 2. 如果交易已经在区块链提交了，就没有写入committed的必要了
	//	无论是否提交过，commit阶段，都要删除cached中的相关消息（Commit阶段，cached一定已经写入或写过相关消息）
	s.CommitPool.deleteTxFromCached(index)
}


// 验证stateTransfer消息，利用msgPayload中的proof字段验证
func (s *ServerShard)validateStateTransferMsg(stateData types.StateData) error{
	// 1. 验证账户是否是freeze状态(没必要)
	// 2. 利用state proof验证消息
	// 获取验证proof必须的参数：rootHash, key, proofDB
	//blockHeight := stateData.StateProof.BlockHeight
	proofs := stateData.StateProof.Proof.(light.NodeList)
	nodeSet := proofs.NodeSet()
	proof := &readTraceDB{db: nodeSet}
	// 根据source shard的height获取rootHash（实际操作时不需要获取）
	rootHash := stateData.StateProof.RootHash
	addr := stateData.State.Address.Bytes() // addr类型转换为[]bytes
	// 参数类型rootHash common.Hash, key []byte, proofDb ethdb.KeyValueReader
	// proofDb: hash->value, 数据结构[][]byte
	_, _, err := trie.VerifyProof(rootHash, addr, proof)
	if err!=nil{
		utils.Logger().Error().Msg("[ProposeHandler] verify failure")
		return errors.WithMessagef(err, "merkle proof verification failed: %v", err)
	}
	return nil
}


// 检查接收到的节点集是否仅包含使证明通过所需的 trie 节点。
type readTraceDB struct {
	db    ethdb.KeyValueReader
	reads map[string]struct{}
}
// Get returns a stored node
func (db *readTraceDB) Get(k []byte) ([]byte, error) {
	if db.reads == nil {
		db.reads = make(map[string]struct{})
	}
	db.reads[string(k)] = struct{}{}
	return db.db.Get(k)
}
// Has returns true if the node set contains the given key
func (db *readTraceDB) Has(key []byte) (bool, error) {
	_, err := db.Get(key)
	return err == nil, nil
}