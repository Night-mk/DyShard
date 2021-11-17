/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/12/21$ 6:09 AM$
 **/
package atomic

import (
	"context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
	"net"
)

// WhiteListChecker intercepts RPC and checks that the caller is whitelisted.
func WhiteListCheckerShard(ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler) (interface{}, error) {
	peerinfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, status.Errorf(codes.Internal, "failed to retrieve peer info")
	}

	host, _, err := net.SplitHostPort(peerinfo.Addr.String())
	if err != nil {
		return nil, status.Errorf(codes.Internal, err.Error())
	}

	serv := info.Server.(*ServerShard)
	if !Includes(serv.Config.Whitelist, host) {
		return nil, status.Errorf(codes.PermissionDenied, "host %s is not in whitelist", host)
	}

	// Calls the handler
	h, err := handler(ctx, req)

	return h, err
}

// Includes checks that the 'arr' includes 'value'
func Includes(arr []string, value string) bool {
	for i := range arr {
		if arr[i] == value {
			return true
		}
	}
	return false
}