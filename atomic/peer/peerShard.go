/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/30/21$ 3:35 AM$
 **/
package peer

import (
	"context"
	"fmt"
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	"github.com/pkg/errors"
	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"time"
)

type CommitClientShard struct {
	Connection c_pb.CommitServiceClient
}

// New creates instance of peer client.
// 'addr' is a coordinator network address (host + port).
func NewClient(addr string) (*CommitClientShard, error) {
	var (
		conn *grpc.ClientConn
		err  error
	)
	// 增加最小超时时间MinConnectTimeout
	connParams := grpc.ConnectParams{
		Backoff: backoff.Config{
			BaseDelay: 100 * time.Millisecond,
			MaxDelay:  5 * time.Second,
		},
		MinConnectTimeout: 500 * time.Millisecond,
	}
	//var kacp = keepalive.ClientParameters{
	//	Time:                10 * time.Second, // send pings every 10 seconds if there is no activity
	//	Timeout:             time.Second,      // wait 1 second for ping ack before considering the connection dead
	//	PermitWithoutStream: true,             // send pings even without active streams
	//}
	//conn, err = grpc.Dial(addr, grpc.WithConnectParams(connParams), grpc.WithInsecure(), grpc.WithKeepaliveParams(kacp))
	conn, err = grpc.Dial(addr, grpc.WithConnectParams(connParams), grpc.WithInsecure())
	if err != nil {
		time.Sleep(10*time.Millisecond)
		fmt.Println("[NewClient]: failed to connect")
		return nil, errors.Wrap(err, "failed to connect")
	}
	return &CommitClientShard{Connection: c_pb.NewCommitServiceClient(conn)}, nil
}

// 使用HTTP版本grpc做尝试（client是没变吗= =）
func NewClientHttp(addr string) (*CommitClientShard, error){
	var (
		conn *grpc.ClientConn
		err  error
	)
	conn, err = grpc.Dial(addr, grpc.WithInsecure())
	if err != nil {
		time.Sleep(10*time.Millisecond)
		fmt.Println("[NewClient]: failed to connect")
		return nil, errors.Wrap(err, "failed to connect")
	}
	return &CommitClientShard{Connection: c_pb.NewCommitServiceClient(conn)}, nil
}

func (client *CommitClientShard) ProposeTwoPhase(ctx context.Context, req *c_pb.ProposeRequest) (*c_pb.ProposeResponse, error) {
	//fmt.Println("[Coordinator] execute client propose: [index=",req.Index,"]")
	return client.Connection.ProposeTwoPhase(ctx, req)
}


func (client *CommitClientShard) CommitTwoPhase(ctx context.Context, req *c_pb.CommitRequest) (*c_pb.Response, error) {
	//fmt.Println("[Coordinator] execute client commit: [index=",req.Index,"]")
	return client.Connection.CommitTwoPhase(ctx, req)
}

// Get queries value of specific key
func (client *CommitClientShard) Get(ctx context.Context, key string) (*c_pb.Value, error) {
	return client.Connection.Get(ctx, &c_pb.Msg{Key: key})
}

/**
	dynamic sharding修改
**/
// StateTransfer将一组state从 source followers转到 target followers
// 2PC StateTransfer client端协议
func (client *CommitClientShard) StateTransfer(ctx context.Context, source uint32, target uint32, msgPayload []byte, index uint64) (*c_pb.Response, error) {
	//fmt.Println("execute client stateTransfer")
	return client.Connection.StateTransfer(ctx, &c_pb.TransferRequest{Source: source, Target: target, MsgPayload: msgPayload, Index: index})
}