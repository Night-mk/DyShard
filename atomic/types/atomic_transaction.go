/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/8/21$ 12:42 AM$
 **/
package types

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/harmony-one/harmony/crypto/hash"
	"github.com/harmony-one/harmony/shard"
	"io"
	"math/big"
	"sync/atomic"
)

/**
	定义状态迁移消息类型
 */
type MsgType uint32
const(
	STATE_TRANSFER MsgType=iota
)

// 账户状态
type state struct {
	Address common.Address
	Balance *big.Int
	Nonce uint64 // 账户的nonce值也要设置
	Data []byte // 表示合约
}
// 状态证明
type stateProof struct {
	BlockHeight uint64
	RootHash common.Hash
	Proof interface{}
}
// 状态数据
type StateData struct {
	State state
	StateProof stateProof
}
// 状态迁移交易StateTransferMsg
type StateTransferMsg struct {
	Source uint32
	Target uint32
	MsgPayload []byte
	Index uint64 // 消息编号
}


/**
	定义一种新的交易类型：状态迁移交易
 */
type StateTransferTransaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
}
// 特殊交易类型，不需要签名、消耗gas
type txdata struct {
	SourceShardID uint32
	TargetShardID uint32
	SourceHeight uint64 // Source shardID锁定数据时的区块高度

	Address common.Address
	Balance *big.Int
	Nonce uint64 // 账户的nonce值也要设置
	Data []byte // 表示合约

	ToFreeze bool  // 参数用于判断是否是锁定state的交易
	// This is only used when marshaling to JSON.
	Hash *common.Hash `json:"hash" rlp:"-"`
	Index uint64
}

func NewStateTransferTransaction(source uint32, target uint32, height uint64, address common.Address, balance *big.Int, nonce uint64, data []byte, toFreeze bool, index uint64) *StateTransferTransaction{
	newTx := &StateTransferTransaction{
		data: txdata{
			source,
			target,
			height,
			address,
			balance,
			nonce,
			data,
			toFreeze, // default: true
			nil,
			index,
		}}

	return newTx
}

func NewStateData(addr common.Address, balance *big.Int, nonce uint64, data []byte, bh uint64, roothash common.Hash, proof []byte) StateData{
	s := StateData{
		State: state{
			addr,
			balance,
			nonce,
			data,
		},
		StateProof: stateProof{
			bh,
			roothash,
			proof,
		},
	}
	return s
}

// 深拷贝方法,针对单个StateTransferTransaction
func (s *StateTransferTransaction) Copy() *StateTransferTransaction{
	if s == nil {
		return nil
	}
	var cpy StateTransferTransaction
	cpy.data.CopyFrom(&s.data)
	return &cpy
}

// 对交易进行深拷贝
func (d *txdata) CopyFrom(d2 *txdata){
	d.SourceShardID = d2.SourceShardID
	d.TargetShardID = d2.TargetShardID
	d.SourceHeight = d2.SourceHeight
	d.Address = d2.Address
	d.Nonce = d2.Nonce
	d.Data = d2.Data
	d.ToFreeze = d2.ToFreeze
	d.Hash = copyHash(d2.Hash)
	d.Index = d2.Index
}
// 复制hash
func copyHash(hash *common.Hash) *common.Hash {
	if hash == nil {
		return nil
	}
	copy := *hash
	return &copy
}
// 复制state
func copyState(s *state) *state{
	var d state
	d.Address = s.Address
	d.Data = s.Data
	d.Nonce = s.Nonce
	d.Balance = copyBigInt(s.Balance) // 必须初始化一个值
	return &d
}

func copyAddr(addr *common.Address) *common.Address {
	if addr == nil {
		return nil
	}
	copy := *addr
	return &copy
}

func copyBigInt(num *big.Int) *big.Int{
	if num == nil{
		return nil
	}
	copy := new(big.Int).Set(num)
	return copy
}

// rlp 编码解码，对StateTransferTransaction编码解码只用对其中的data编码解码
// EncodeRLP
func (tx *StateTransferTransaction) EncodeRLP(w io.Writer) error{
	return rlp.Encode(w, &tx.data)
}

// DecodeRLP 解码时会计算storage的size
func (tx *StateTransferTransaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(common.StorageSize(rlp.ListSize(size)))
	}
	return err
}

// Hash返回tx的RLP编码的哈希值
func (s *StateTransferTransaction) Hash() common.Hash{
	if hash := s.hash.Load(); hash != nil {
		return hash.(common.Hash)
	}
	v := hash.FromRLP(s) // 返回交易的RLP编码的哈希值
	s.hash.Store(v)
	return v
}

// Size 返回tx的RLP编码后的存储打大小
// Size returns the true RLP encoded storage size of the transaction, either by
// encoding and returning it, or returning a previously cached value.
func (tx *StateTransferTransaction) Size() common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(common.StorageSize(c))
	return common.StorageSize(c)
}

type writeCounter common.StorageSize

func (c *writeCounter) Write(b []byte) (int, error) {
	*c += writeCounter(len(b))
	return len(b), nil
}

/**
	GET 方法
 */
func (s *StateTransferTransaction) TargetShardID() uint32 {
	return s.data.TargetShardID
}
func (s *StateTransferTransaction) SourceShardID() uint32 {
	return s.data.SourceShardID
}
func (s *StateTransferTransaction) SourceHeight() uint64 {
	return s.data.SourceHeight
}
//func (s *StateTransferTransaction) State() *state {
//	return s.data.State
//}
func (s *StateTransferTransaction) ToFreeze() bool {
	return s.data.ToFreeze
}
func (s *StateTransferTransaction) Index() uint64 {
	return s.data.Index
}

func (s *StateTransferTransaction) Address() common.Address {
	return s.data.Address
}

func (s *StateTransferTransaction) Nonce() uint64 {
	return s.data.Nonce
}

func (s *StateTransferTransaction) Balance() *big.Int {
	return s.data.Balance
}

func (s *StateTransferTransaction) Data() []byte {
	return s.data.Data
}

/*
	SET 方法
 */
func (s *StateTransferTransaction) SetToFreeze(label bool)  {
	s.data.ToFreeze = label
}

func (s *StateTransferTransaction) SetAddress(addr common.Address)  {
	s.data.Address = addr
}

func (s *StateTransferTransaction) SetNonce(nonce uint64)  {
	s.data.Nonce = nonce
}

func (s *StateTransferTransaction) SetBalance(b *big.Int)  {
	s.data.Balance = b
}

func (s *StateTransferTransaction) SetData(d []byte)  {
	s.data.Data = d
}

func (s *StateTransferTransaction) SetState(address common.Address, balance *big.Int, nonce uint64, data []byte){
	s.data.Address = address
	s.data.Balance = balance
	s.data.Nonce = nonce
	s.data.Data = data
}

//func (s *StateTransferTransaction) SetState(data StateData)  {
//	s.data.State.Address = data.State.Address
//	s.data.State.Balance = data.State.Balance
//	s.data.State.Nonce = data.State.Nonce
//	s.data.State.Data = data.State.Data
//}


// StateTransferTransactions是交易的slice type.
type StateTransferTransactions []*StateTransferTransaction

/*
	复数交易的相关函数
 */
func (ss StateTransferTransactions) Len() int {return len(ss)}

// Copy makes a deep copy of the receiver.
func (ss StateTransferTransactions) Copy() (cpy StateTransferTransactions) {
	for _, r := range ss {
		cpy = append(cpy, r.Copy())
	}
	return cpy
}

// Len() 和 GetRlp() 实现了DerivableBase接口
// GetRlp implements Rlpable and returns the i'th element of s in rlp.
func (ss StateTransferTransactions) GetRlp(i int) []byte {
	if len(ss) == 0 {
		return []byte{}
	}
	enc, _ := rlp.EncodeToBytes(ss[i])
	return enc
}

// TargetShardID returns the destination shardID of the StateTransferTransaction
func (ss StateTransferTransactions) TargetShardID(i int) uint32 {
	if len(ss) == 0 {
		return shard.IniShardID
	}
	return ss[i].data.TargetShardID
}

func (ss StateTransferTransactions) SourceShardID(i int) uint32 {
	if len(ss) == 0 {
		return shard.IniShardID
	}
	return ss[i].data.SourceShardID
}

