/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/13/21$ 5:35 PM$
 **/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	"github.com/harmony-one/harmony/atomic/peer"
	"github.com/harmony-one/harmony/atomic/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"io/ioutil"
	"math/big"
	"sync"
	"time"
)

const (
	TXNUM = 1 // 消息总量
	ParallelChNum = 1500 // 并发协程数量
	//CoordinatorAddr = "127.0.0.1:4000"
	CoordinatorAddr = "10.20.6.199:4000" // server
	FollowerAddr = "localhost:4001"
	UsedAddrIndex = 0
)

var (
	whitelist = []string{"127.0.0.1"}
	//testTable = obtainTestData(shardList)
)

// 读取json文件最重要的就是这个格式，struct的首字母要大写，并且后面要加上json里的key
type AccountJson struct {
	PrivateKey string `json:"privateKey"`
	PublicKey string `json:"publicKey"`
	Address string `json:"address"`
	Bech32Addr string `json:"bech32Addr"`
}

// 测试数据准备
// 1. 从json文件读取address数据
//var AddressList = []string{
//	"0x316e11c89d8ec90370a9e4cc6568dc78aef4e11b",
//	"0x549c74a16ddd5c7e63d48cb6ccd6ee3e97677842",
//	"0x7e0070db0502e1afa90a30d8f14b97bb7b7e2174",
//}

var shardList = []uint32{
	uint32(0), uint32(1), uint32(2), uint32(3),
}

// 设置测试数据
func obtainTestDataState(shard []uint32, addressList []string) []types.StateTransferMsg {
	var testTable = make([]types.StateTransferMsg, len(addressList))
	fmt.Println("address ori: ", addressList)
	// 获取stateData的rlp编码
	for i, addr := range addressList{
		// 构建结构体StateData
		state := types.NewStateData(common.HexToAddress(addr), big.NewInt(0), 0, []byte{}, 0, common.Hash{}, []byte{})
		// rlp编码
		enState, _ := rlp.EncodeToBytes(state)
		//fmt.Printf("%v → %X\n", state, enState)
		var deState types.StateData
		err := rlp.DecodeBytes(enState, &deState)
		fmt.Printf("State:err=%+v,val=%+v\n", err, deState)
		stMsg := types.StateTransferMsg{
			Source: shard[0],
			Target: shard[1],
			MsgPayload: enState,
			Index: uint64(i+1),
		}
		testTable[i] = stMsg
	}
	return testTable
}


// 先读取成string
func readFile(filename string) string {
	b, err := ioutil.ReadFile(filename) // just pass the file name
	if err != nil {
		fmt.Print(err)
	}
	str := string(b) // convert content to a 'string'
	//fmt.Println(str) // print the content as a 'string'
	return str
}
func deserializeJson(configJson string) []AccountJson {
	jsonAsBytes := []byte(configJson)
	configs := make([]AccountJson, 0)
	err := json.Unmarshal(jsonAsBytes, &configs)
	//fmt.Printf("%#v", configs)
	if err != nil {
		panic(err)
	}
	return configs
}

// 准备测试数据
func obtainTestData(shard []uint32) []types.StateTransferMsg {
	//var addressList = []string{
	//	"0x316e11c89d8ec90370a9e4cc6568dc78aef4e11b",
	//	"0x549c74a16ddd5c7e63d48cb6ccd6ee3e97677842",
	//	"0x7e0070db0502e1afa90a30d8f14b97bb7b7e2174",
	//}
	// 从json文件读取数据
	filename := "accounts-transfer.json"
	readFile(filename)
	accStr := readFile(filename)
	accSlice := deserializeJson(accStr)
	var addressList []string
	addressNum := 0
	for index, item := range accSlice{
		// 判断地址是否已经发送过
		if index<UsedAddrIndex {continue}
		addressList = append(addressList, item.Address)
		addressNum += 1
		if addressNum>=TXNUM {break}
	}
	var testTable = make([]types.StateTransferMsg, len(addressList))
	fmt.Println("address ori: ", len(addressList))
	fmt.Println("address ori: ", addressList)
	// 获取stateData的rlp编码
	for i, addr := range addressList{
		// 将address转变为common.Address再转成byte形式
		byteAddr := common.HexToAddress(addr)
		stMsg := types.StateTransferMsg{
			Source: shard[0],
			Target: shard[1],
			MsgPayload: byteAddr.Bytes(),
			Index: uint64(i+1+UsedAddrIndex), // 为了测试方便，index要加上UsedAddrIndex
		}
		testTable[i] = stMsg
	}
	// 构建更多交易
	return testTable
}

// 客户端，准备大量交易，并行发给coordinator调用状态迁移协议
func ParallelTest(){
	testTable := obtainTestData(shardList)
	client, err := peer.NewClient(CoordinatorAddr)
	if err != nil {
		panic(err)
	}
	time.Sleep(2* time.Second)
	fmt.Println("========= Create CLIENT END =========", len(testTable))

	// 控制coordinator并发的协程数量
	coordPCh := make(chan struct{}, ParallelChNum)
	defer close(coordPCh)

	startTime := time.Now() // 获取当前时间
	// 调用stateTransfer请求
	// 使用goroutine并行调用
	var wg sync.WaitGroup
	for index, state := range testTable {
		wg.Add(1)
		coordPCh <- struct{}{}
		go func(s types.StateTransferMsg, i int) {
			defer wg.Done()
			fmt.Println("client send index=",s.Index)
			resp, err := client.StateTransfer(context.Background(), s.Source, s.Target, s.MsgPayload, s.Index)

			if err != nil {
				log.Error(err.Error())
			}
			if resp.Acktype != c_pb.AckType_ACK {
				fmt.Println("err response type: ", resp.Acktype)
				log.Error(err.Error())
			}
			<- coordPCh
		}(state, index+1)
	}

	wg.Wait()
	fmt.Println("=========== STATETRANSFER END ===========")
	elapsed := time.Since(startTime)

	fmt.Println("===== TX NUM:",len(testTable),"=====")
	// 计算纳秒
	duration := elapsed.Seconds()
	tps := TXNUM /duration
	fmt.Println("2PC time consumed: ", duration)
	fmt.Println("2PC TPS: ", tps)
}

// 测试直接for循环发送消息
func SingleTest(){
	testTable := obtainTestData(shardList)
	client, err := peer.NewClient(CoordinatorAddr)
	if err != nil {
		panic(err)
	}
	time.Sleep(5* time.Second)
	fmt.Println("========= Create CLIENT END =========", len(testTable))

	for _, s := range testTable {
		resp, err := client.StateTransfer(context.Background(), s.Source, s.Target, s.MsgPayload, s.Index)
		fmt.Println("client send index=",s.Index)
		if err != nil {
			log.Error(err.Error())
		}
		if resp.Acktype != c_pb.AckType_ACK {
			fmt.Println("err response type: ", resp.Acktype)
			log.Error(err.Error())
		}
	}
}

// 单独测试是否能调用follower
func FollowerConnTest(){
	//client, err := peer.NewClient(FollowerAddr)
	client, err := peer.NewClientHttp(FollowerAddr)
	if err != nil {
		panic(err)
	}
	time.Sleep(2* time.Second)
	fmt.Println("========= Create CLIENT END =========")

	var kacp = keepalive.ClientParameters{
		Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
		Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
		PermitWithoutStream: true,             // send pings even without active streams
	}
	//Dial 中传入 keepalive 配置
	_, err = grpc.Dial(FollowerAddr, grpc.WithInsecure(), grpc.WithKeepaliveParams(kacp))
	// 测试连接
	//_, err = grpc.Dial(FollowerAddr, grpc.WithInsecure())
	if err != nil {
		fmt.Println("network error", err)
		return
	}

	// 准备数据
	payload := "0x549c74a16ddd5c7e63d48cb6ccd6ee3e97677842"
	pRequest := &c_pb.ProposeRequest{Source: 0, Target: 1, MsgPayload: common.HexToAddress(payload).Bytes(), CommitType: c_pb.CommitType_TWO_PHASE_COMMIT, Index: 0}
	var response *c_pb.ProposeResponse
	//var maxAttemptNum = 2
	// 循环发送测试
	//for (response == nil && maxAttemptNum>0) || (response != nil && response.Acktype == c_pb.AckType_NACK) {
	response, err = client.ProposeTwoPhase(context.Background(), pRequest)
	fmt.Println("=====Coordinator Server Response: ", response)
	if err != nil {
		fmt.Errorf("[Coordinator Server Error]: %v", err)
	}
		//maxAttemptNum -= 1
	//}
}

func main(){
	ParallelTest()
	//FollowerConnTest()
	//SingleTest()
	//fmt.Println(testTable)
}


