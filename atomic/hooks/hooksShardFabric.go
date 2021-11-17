/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/9/21$ 8:17 AM$
 **/
package hooks

import (
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	c_atomic "github.com/harmony-one/harmony/atomic"
	"github.com/harmony-one/harmony/atomic/hooks/src"
)

type ProposeShardHook func(req *c_pb.ProposeRequest) bool
type CommitShardHook func(req *c_pb.CommitRequest) bool

func GetHookF() ([]c_atomic.OptionShard, error) {
	proposeShardHook := func(f ProposeShardHook) func(*c_atomic.ServerShard) error {
		return func(server *c_atomic.ServerShard) error {
			server.ProposeShardHook = f
			return nil
		}
	}

	commitShardHook := func(f CommitShardHook) func(*c_atomic.ServerShard) error {
		return func(server *c_atomic.ServerShard) error {
			server.CommitShardHook = f
			return nil
		}
	}

	return []c_atomic.OptionShard{proposeShardHook(src.ProposeShard), commitShardHook(src.CommitShard)}, nil
}