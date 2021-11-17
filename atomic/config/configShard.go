/**
 * @Author Night-mk
 * @Description //TODO
 * @Date 8/30/21$ 3:41 AM$
 **/
package config

import (
	"flag"
	"strconv"
	"strings"
)

type ConfigShard struct {
	Role        string
	Nodeaddr    string
	Coordinator string
	Followers   map[uint32][]string
	Whitelist   []string
	CommitType  string
	Timeout     uint64
}

// 定义followershard类型，需要函数 String,  Set
// flag包用来解析命令行选项
type followerShard []string

func(f *followerShard) String() string{
	return strings.Join(*f, ",")
}

func(f *followerShard) Set(value string) error{
	*f = append(*f, value)
	return nil
}



// Get creates configuration from yaml configuration file (if '-config=' flag specified) or command-line arguments.
func GetConfig() *ConfigShard {
	// command-line flags
	// 定义命令行参数对应的变量，默认只有三种：flag.String, flag.Int, flag.Bool
	// 也可以自定义
	role := flag.String("role", "follower", "role (coordinator of follower)")
	nodeaddr := flag.String("nodeaddr", "", "node address")
	coordinator := flag.String("coordinator", "", "coordinator address")
	committype := flag.String("committype", "three-phase", "two-phase or three-phase commit mode")
	timeout := flag.Uint64("timeout", 1000, "ms, timeout after which the message is considered unacknowledged (only for three-phase mode, because two-phase is blocking by design)")
	followerShard := flag.String("followers", "", "follower's addresses")
	whitelist := flag.String("whitelist", "127.0.0.1", "allowed hosts")
	flag.Parse()

	// 将string类型的followerShard转换为 map[uint32][]string类型放在输出中
	//followersArray := strings.Split(*followerShard, ",")
	shardArray := strings.Split(*followerShard, ",")
	followersArray := make(map[uint32][]string)
	if *role == "coordinator"{
		for _, item := range shardArray{
			follower := strings.SplitN(item, "+", 2)
			intNum, _ := strconv.Atoi(follower[0])
			followersArray[uint32(intNum)]=append(followersArray[uint32(intNum)], follower[1])
		}
	} else { // follower节点也要添加自己
		follower := strings.SplitN(*nodeaddr, "+", 2)
		intNum, _ := strconv.Atoi(follower[0])
		followersArray[uint32(intNum)]=append(followersArray[uint32(intNum)], follower[1])
		//fmt.Println("configShard_nodeaddr: ",*nodeaddr)
		//fmt.Println("configShard_nodeaddr: ",follower[0])
		//fmt.Println("configShard_nodeaddr: ",follower[1])
	}

	whitelistArray := strings.Split(*whitelist, ",")

	return &ConfigShard{*role, *nodeaddr, *coordinator,
		followersArray, whitelistArray, *committype,
		*timeout}

}