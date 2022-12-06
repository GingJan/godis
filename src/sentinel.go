package src

import "time"

type sentinelState struct {
	currentEpoch uint64//当前纪元，用于故障转移
	master *dict//key是master的名字，val是该master sentinelRedisInstance实例的地址/指针
	tilt int


}

type sentinelRedisInstance struct {//该结构用于哨兵模式，由哨兵负责维护
	flags int//实例类型及其当前状态，类型可以是master，slave，sentinel
	name string//实例名字，主服务名字由用户在配置文件指定，从服务器及哨兵名字由哨兵自动设置，一般未ip:port

	runid string//实例运行id
	configEpoch uint64//配置的纪元
	addr *sentinelAddr//本实例代表的主从或哨兵的地址

	downAfterPeriod time.Duration//实例无响应x毫秒后 认为主观下线


	failoverTimeout time.Duration

	/*master使用的字段*/
	sentinels *dict//监视同一个master的其他哨兵，
	slaves *dict//key=>从ip+port，val=>从的sentinelRedisInstance实例，该master的从
	quorum int//客观下线需要的票数
	parallelSync int//在进行failover时，可同时向新master进行同步的slave数量

	/*slave使用的字段*/
	slavePriority int//从 优先级
	master *sentinelRedisInstance//从的master
	slaveMasterHost string
	slaveMasterPort int
	slaveMasterLinkStatus int
	slaveReplOffset uint64

	/*failover故障转移的字段*/
	leader string//如果本实例是master，则这是执行故障转移的哨兵的runid，如果本实例是哨兵，这是本哨兵投票进行failover的哨兵runid
	leaderEpoch int


}

type sentinelAddr struct {
	ip string
	port int
}

type