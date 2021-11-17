/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 9/13/21$ 5:23 PM$
 **/
package main

import (
	atomic_server "github.com/harmony-one/harmony/atomic"
	"github.com/harmony-one/harmony/atomic/config"
	"github.com/harmony-one/harmony/atomic/hooks"
	nodeconfig "github.com/harmony-one/harmony/internal/configs/node"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	conf_ori := config.GetConfig()
	// 将获取的cmd输入转换为TwoPcConfig类型
	conf := nodeconfig.TwoPcConfig{
		Role: conf_ori.Role,
		Nodeaddr: conf_ori.Nodeaddr,
		Coordinator: conf_ori.Coordinator,
		Followers: conf_ori.Followers,
		Whitelist: conf_ori.Whitelist,
		CommitType: conf_ori.CommitType,
		Timeout: int(conf_ori.Timeout),
	}
	commitPool := &atomic_server.CommitPool{}
	hooks, err := hooks.GetHookF()
	if err != nil {
		panic(err)
	}
	s, err := atomic_server.NewCommitServerShard(conf, commitPool, hooks...)
	if err != nil {
		panic(err)
	}

	// Run函数启动一个non-blocking GRPC server
	//s.Run(atomic_server.WhiteListCheckerShard)
	s.RunHttp()
	<-ch // 如果一直没有系统信号，就一直等待？
	s.StopHttp()
}
