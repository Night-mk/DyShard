/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/9/21$ 8:17 AM$
 **/
package src

import (
	c_pb "github.com/harmony-one/harmony/api/proto/commit"
	"github.com/harmony-one/harmony/internal/utils"
)

func ProposeShard(req *c_pb.ProposeRequest) bool {
	utils.Logger().Printf("propose hook on tx index %d is OK", req.Index)
	//log.Infof("propose hook on tx index %d is OK", req.Index)
	return true
}

func CommitShard(req *c_pb.CommitRequest) bool {
	utils.Logger().Printf("commit hook on tx index %d is OK", req.Index)
	//log.Infof("commit hook on tx index %d is OK", req.Index)
	return true
}