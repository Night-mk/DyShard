/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/30/21$ 2:01 AM$
 **/
package atomic

import (
	"context"
	"fmt"
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	"github.com/harmony-one/harmony/atomic/peer"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"github.com/harmony-one/harmony/internal/utils"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

type OptionShard func(server *ServerShard) error
const COORDINATOR_SHARDID = uint32(10000)
const (
	TWO_PHASE   = "two-phase"
	THREE_PHASE = "three-phase"
)

type ServerShard struct {
	c_pb.UnimplementedCommitServiceServer
	Addr                 string
	ShardID              uint32 // 初始化时需要整一个和node一样的shardID
	FollowerType         c_pb.FollowerType
	Followers            []*peer.CommitClientShard
	ShardFollowers       map[uint32][]*peer.CommitClientShard // 带shard的全体follower集合
	Config               *nodeconfig.TwoPcConfig // 这个config改为node里的配置
	GRPCServer           *grpc.Server
	ProposeShardHook     func(req *c_pb.ProposeRequest) bool
	CommitShardHook      func(req *c_pb.CommitRequest) bool
	CommitPool           *CommitPool // 没初始化上？

	CoordinatorCache     *Cache
	cancelCommitOnHeight map[uint64]bool
	mu                   sync.RWMutex
}

// 2PC 第一阶段
func (s *ServerShard) ProposeTwoPhase(ctx context.Context, req *c_pb.ProposeRequest) (*c_pb.ProposeResponse, error) {
	//fmt.Println("[Server] ",s.Addr, " execute follower propose")
	// 设置在某个Height是否需要cancel
	s.SetProgressForCommitPhase(req.Index, false)

	return s.ProposeHandler(ctx, req, s.ProposeShardHook)
}

// 2PC 第二阶段
func (s *ServerShard) CommitTwoPhase(ctx context.Context, req *c_pb.CommitRequest) (resp *c_pb.Response, err error) {
	//fmt.Println("[Server] execute follower commit: [index=",req.Index,"]")
	// 两阶段commit
	resp, err = s.CommitHandler(ctx, req, s.CommitShardHook)
	if err != nil {
		return
	}

	return
}

func (s *ServerShard) Get(ctx context.Context, req *c_pb.Msg) (*c_pb.Value, error) {
	// 从database通过key获取数据
	//value, err := s.DB.Get(req.Key)
	value := []byte{}
	//if err != nil {
	//	return nil, err
	//}
	return &c_pb.Value{Value: value}, nil
}


/**
	stateTransfer 并行化方案
*/
func (s *ServerShard) StateTransfer(ctx context.Context, req *c_pb.TransferRequest) (*c_pb.Response, error){
	//fmt.Println("=================START stateTransfer Protocol=================")
	var (
		response *c_pb.Response
		err      error
	)
	// 选择使用的协议，目前暂定只用2PC(propose, commit)
	var ctype c_pb.CommitType
	if s.Config.CommitType == THREE_PHASE {
		ctype = c_pb.CommitType_THREE_PHASE_COMMIT
	} else {
		ctype = c_pb.CommitType_TWO_PHASE_COMMIT
	}

	/**=======2PC协议流程=======*/
	// propose阶段
	//fmt.Println("[StateTransfer] Propose phase, index=", req.Index)
	//fmt.Printf("[StateTransfer] Propose phase: index=[%d] source=[%d], target=[%d], address=[%+v] \n", req.Index, req.Source, req.Target, common.BytesToAddress(req.MsgPayload))

	// 根据source和target选择followers组
	//currentFollowers := s.ObtainFollowers(req.Source, req.Target)
	currentFollowers := s.ObtainFollowersLimit(req.Source, req.Target)
	//fmt.Println("=============Followers============",currentFollowers)
	fmt.Println("Follower num: ",len(currentFollowers))
	// 只用发送给2/3的validator执行2PC
	//realFollowerNum := int(math.Ceil(float64(2*(len(currentFollowers)/3))))

	// 对每个follower并发处理
	var wg sync.WaitGroup
	proposeResChan := make(chan *c_pb.ProposeResponse, 1000)
	proposeErrChan := make(chan error, 1000)

	// TIME: 计算state transfer tx的调用时间
	startTime := time.Now() // 获取当前时间

	for _, follower := range currentFollowers {
		wg.Add(1)
		go func(follower *peer.CommitClientShard) {
			var (
				responseP *c_pb.ProposeResponse
				err      error
			)
			// 设置最大重发次数？
			var maxAttemptNum = 1
			for (responseP == nil && maxAttemptNum>0) || (responseP != nil && responseP.Acktype == c_pb.AckType_NACK) {
				// 每个follower（source, target）用grpc调用coordinator给他们发送消息
				responseP, err = follower.ProposeTwoPhase(ctx, &c_pb.ProposeRequest{Source: req.Source, Target: req.Target, MsgPayload: req.MsgPayload, CommitType: ctype, Index: req.Index})
				//fmt.Println("=====Coordinator Server Response: ", responseP)
				//if err != nil {
				//	fmt.Errorf("[Coordinator Server Error]: %v", err)
				//}
				maxAttemptNum -= 1
			}

			if err != nil || responseP == nil {
				fmt.Println("[Coordinator Server]: Propose Failure")
				fmt.Printf("[Coordinator Server Error]: %v", err)
				return
			}
			if responseP.Acktype != c_pb.AckType_ACK {
				utils.Logger().Info().
					Msg("follower not acknowledged msg")
				return
			}
			proposeResChan <- responseP
			proposeErrChan <- err
			// coordinator server设置cache
			// index交易的绝对编号
			s.CoordinatorCache.Set(req.Index, req.MsgPayload)
			wg.Done()
		}(follower)
	}

	// 等待所有follower反馈完成，再进入commit过程
	wg.Wait()
	close(proposeErrChan)
	close(proposeResChan)

	for err := range proposeErrChan{
		//fmt.Println("=============execute proposeErrChan=====================")
		if err != nil {
			utils.Logger().Error().
				Err(err).
				Msg("=============execute proposeErrChan=====================")
			return &c_pb.Response{Acktype: c_pb.AckType_NACK}, nil
		}
	}
	// 处理ProposeResponse的消息
	// 1. 根据一条response构建一个commit阶段的request
	var commitRequest *c_pb.CommitRequest
	writeOnce := 1
	for res := range proposeResChan{
		//fmt.Println("=============execute proposeResChan=====================")
		if res.Acktype != c_pb.AckType_ACK {
			return nil, status.Error(codes.Internal, "follower not acknowledged msg")
		}
		if writeOnce == 1 && len(res.MsgPayload)>0{ // 将其中一个回复的数据写入到commitRequest（只用写一次, 必须MsgPayload不为空才能写入)
			commitRequest = &c_pb.CommitRequest{
				Source: req.Source, Target: req.Target,
				Index: res.Index, Address: res.Address, MsgPayload: res.MsgPayload,
			}
			writeOnce -= 1
		}
	}
	//fmt.Println("===============PROPOSE END IN coordinator==============")
	// the coordinator got all the answers, so it's time to persist msg and send commit command to followers
	_, ok := s.CoordinatorCache.Get(req.Index)
	if !ok {
		utils.Logger().Error().
			Uint64("stIndex", req.Index).
			Msg("[StateTransfer] can not find msg in cache")
		return nil, status.Error(codes.Internal, "can't to find msg in the coordinator's cache")
	}
	// coordinator server将事务写入DB

	// commit阶段
	//fmt.Printf("[StateTransfer] Commit phase: index=[%d] source=[%d], target=[%d], state=[%s] \n", req.Index, req.Source, req.Target, deState)
	var wg1 sync.WaitGroup
	commitResChan := make(chan *c_pb.Response, 1000)
	commitErrChan := make(chan error, 1000)
	for _, follower := range currentFollowers{
		wg1.Add(1)
		go func(follower *peer.CommitClientShard) {
			// coordinator获取到commit的response
			maxAttemptNum := 1 // 限制循环发送数量
			for (response == nil && maxAttemptNum>0) || (response != nil && response.Acktype == c_pb.AckType_NACK) {
				response, err = follower.CommitTwoPhase(ctx, commitRequest)
				maxAttemptNum -= 1
			}
			commitResChan <- response
			commitErrChan <- err
			if err != nil {
				//log.Errorf(err.Error())
				utils.Logger().Err(err).Msg("CommitResponse ERROR")
				return
			}
			if response.Acktype != c_pb.AckType_ACK {
				return
			}
			wg1.Done()
		}(follower)
	}

	wg1.Wait()
	close(commitResChan)
	close(commitErrChan)

	// 对所有follower节点的返回值进行处理
	for res := range commitResChan{
		if res.Acktype != c_pb.AckType_ACK {
			return nil, status.Error(codes.Internal, "follower not acknowledged msg")
		}
	}
	for err := range commitErrChan{
		if err != nil {
			utils.Logger().Err(err).Msg("CommitResponse ERROR")
			return &c_pb.Response{Acktype: c_pb.AckType_NACK}, nil
		}
	}

	// TIME: 计算state transfer tx的调用时间
	elapsed := time.Since(startTime)
	duration := elapsed.Seconds()
	fmt.Println("2PC State Transfer time consumed: ", duration)

	//log.Infof("committed state for tx_index=[%d] \n", req.Index)
	// 返回调用端最终结果
	return &c_pb.Response{Acktype: c_pb.AckType_ACK}, nil
}

// 获取需要发送的followers
// 这里可以设置follower的数量
func (s *ServerShard)ObtainFollowersLimit(source uint32, target uint32) (follower []*peer.CommitClientShard){
	// 只用发送给2/3的validator执行2PC
	realNumSource := int(math.Ceil(float64(2*(len(s.ShardFollowers[source])/3))))
	realNumTarget := int(math.Ceil(float64(2*(len(s.ShardFollowers[target])/3))))
	for _, node := range s.ShardFollowers[source][:realNumSource+1]{
		follower = append(follower, node)
	}
	for _, node := range s.ShardFollowers[target][:realNumTarget+1]{
		follower = append(follower, node)
	}
	return follower
}

// 获取分片的全部节点作为followers
func (s *ServerShard)ObtainFollowers(source uint32, target uint32) (follower []*peer.CommitClientShard){
	for _, node := range s.ShardFollowers[source]{
		follower = append(follower, node)
	}
	for _, node := range s.ShardFollowers[target]{
		follower = append(follower, node)
	}
	return follower
}

// 根据config，创建server端instance
func NewCommitServerShard(conf nodeconfig.TwoPcConfig, commitPool *CommitPool, opts ...OptionShard) (*ServerShard, error){
	var wg sync.WaitGroup
	wg.Add(1)
	fmt.Println("============2PC CONFIG============", conf)

	// 初始化serverShard
	//server := &ServerShard{Addr: conf.Nodeaddr, DBPath: conf.DBPath}
	server := &ServerShard{}
	// Follower初始化CommitPool
	if conf.Role != "coordinator"{
		server.CommitPool = commitPool
		fmt.Println("===Current Shard ID ===", commitPool.chain.ShardID())
	}
	// 对coordinator和follower分开初始化
	if conf.Role == "coordinator"{
		server.Addr = conf.Nodeaddr
		server.ShardID = COORDINATOR_SHARDID
	}else{
		// follower分解nodeaddr
		addrWithShard := strings.SplitN(conf.Nodeaddr, "+", 2)
		server.Addr = addrWithShard[1]
		intNum, _ := strconv.Atoi(addrWithShard[0])
		server.ShardID = uint32(intNum)
	}

	var err error
	// 查看option是否有问题
	for _, option := range opts {
		err = option(server)
		if err != nil {
			return nil, err
		}
	}

	nodeaddr := conf.Nodeaddr
	if conf.Role != "coordinator"{
		nodeaddr = strings.SplitN(conf.Nodeaddr, "+", 2)[1]
	}
	utils.Logger().Info().Str("nodeAddress: ", nodeaddr)
	//log.Info("[2PC] NodeAddress: ", nodeaddr)

	// 在coordinator中初始化配置里的follower client
	server.ShardFollowers = make(map[uint32][]*peer.CommitClientShard)
	for key, nodes := range conf.Followers {
		for _, node := range nodes{
			fmt.Println("config init node: ", node)
			cli, err := peer.NewClient(node)
			if err != nil {
				return nil, err
			}
			server.Followers = append(server.Followers, cli)
			server.ShardFollowers[key] = append(server.ShardFollowers[key], cli)
		}
	}

	// 在server.config中标记coordinator
	server.Config = &nodeconfig.TwoPcConfig{
		Role: conf.Role,
		Nodeaddr: conf.Nodeaddr,
		Coordinator: conf.Coordinator,
		Followers: conf.Followers,
		Whitelist: conf.Whitelist,
		CommitType: conf.CommitType,
		Timeout: conf.Timeout,
	}
	if conf.Role == "coordinator" {
		server.Config.Coordinator = server.Addr
		server.CoordinatorCache = NewCoorCache()
	}

	// 初始化DB, cache, rollback参数
	//server.DB, err = db.New(conf.DBPath)
	server.cancelCommitOnHeight = map[uint64]bool{}

	if server.Config.CommitType == TWO_PHASE {
		utils.Logger().Info().Msg("two-phase-commit mode enabled")
	} else {
		utils.Logger().Info().Msg("three-phase-commit mode enabled")
	}

	wg.Done()
	wg.Wait()

	return server, err
}

// 查看是否有DB
//func checkServerField(server *ServerShard) error {
//	if server.DB == nil {
//		return errors.New("database is not selected")
//	}
//	return nil
//}

// Run starts non-blocking GRPC server
func (s *ServerShard) Run(opts ...grpc.UnaryServerInterceptor) {
	var err error

	s.GRPCServer = grpc.NewServer(grpc.ChainUnaryInterceptor(opts...))
	c_pb.RegisterCommitServiceServer(s.GRPCServer, s)

	l, err := net.Listen("tcp", s.Addr)
	if err != nil {
		fmt.Println("2PC RUN failed to listen")
		utils.Logger().Fatal().Err(err).Msg("2PC RUN failed to listen")
	}
	fmt.Printf("2PC RUN listening on tcp://%s \n", s.Addr)
	utils.Logger().Printf("2PC RUN listening on tcp://%s", s.Addr)

	go s.GRPCServer.Serve(l)
}

// Stop stops server
func (s *ServerShard) Stop() {
	// stop的时候关闭commitPool.Signal的通道
	s.CommitPool.CloseAllSignal()
	utils.Logger().Info().Msg("2PC stopping server")
	//log.Info("stopping server")
	s.GRPCServer.GracefulStop()
	fmt.Println("2PC server stopped")
	utils.Logger().Info().Msg("2PC server stopped")
}

//func (s *ServerShard) rollback() {
//	s.NodeCache.Delete(s.Height)
//}

func (s *ServerShard) SetProgressForCommitPhase(height uint64, docancel bool) {
	s.mu.Lock()
	s.cancelCommitOnHeight[height] = docancel
	s.mu.Unlock()
}

func (s *ServerShard) HasProgressForCommitPhase(height uint64) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cancelCommitOnHeight[height]
}

// 使用http方式启动节点
func (s *ServerShard) RunHttp(){
	// 在newServer上加上长连接参数 (否则可能连接时长不够报错 code = Unavailable desc = transport is closing)
	//var kaep = keepalive.EnforcementPolicy{
	//	MinTime:             5 * time.Second, // If a client pings more than once every 5 seconds, terminate the connection
	//	PermitWithoutStream: true,            // Allow pings even when there are no active streams
	//}
	//var kasp = keepalive.ServerParameters{
	//	MaxConnectionIdle:     15 * time.Second, // If a client is idle for 15 seconds, send a GOAWAY
	//	MaxConnectionAge:      30 * time.Second, // If any connection is alive for more than 30 seconds, send a GOAWAY
	//	MaxConnectionAgeGrace: 5 * time.Second,  // Allow 5 seconds for pending RPCs to complete before forcibly closing connections
	//	Time:                  5 * time.Second,  // Ping the client if it is idle for 5 seconds to ensure the connection is still active
	//	Timeout:               1 * time.Second,  // Wait 1 second for the ping ack before assuming the connection is dead
	//}
	//s.GRPCServer = grpc.NewServer(grpc.KeepaliveEnforcementPolicy(kaep), grpc.KeepaliveParams(kasp))
	s.GRPCServer = grpc.NewServer()
	c_pb.RegisterCommitServiceServer(s.GRPCServer, s)
	go http.ListenAndServe(s.Addr, grpcHandlerFunc(s.GRPCServer))
	fmt.Printf("2PC RUN listening on http://%s \n", s.Addr)
}

func grpcHandlerFunc(grpcServer *grpc.Server) http.Handler {
	return h2c.NewHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		grpcServer.ServeHTTP(w, r)
	}), &http2.Server{})
}

// 使用http方式停止节点
func (s *ServerShard) StopHttp() {
	// stop的时候关闭commitPool.Signal的通道
	s.CommitPool.CloseAllSignal()
	utils.Logger().Info().Msg("2PC stopping server")
	s.GRPCServer.GracefulStop()
	fmt.Println("2PC server stopped")
	utils.Logger().Info().Msg("2PC server stopped")
}