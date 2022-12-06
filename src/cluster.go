package src

const (
	REDIS_CLUSTER_SLOTS = 16384//2^14
)
type clusterMsg struct {
	totlen uint32 //消息的长度（包含消息头+消息体）
	mtype uint16//消息类型
	count uint16 //消息正文包含的结点信息数量，只在MEET PING PONG三种类型Gossip协议消息时使用
	currentEpoch uint64 //发送者当前纪元，当本纪元的从没活动半数+1的票时，纪元+1，重新投票选举新主
	configEpoch uint64 //如果是主，则=发送者当前纪元，是从，则记录的是它复制的主的纪元
	sender string//发送者名字
	myslots [REDIS_CLUSTER_SLOTS/8]uint8//发送者当前的slot信息
	slaveof string
	port uint16
	flags uint16
	state uint8//所处集群状态
	data clusterMsgData
}

type clusterMsgData struct {
	ping struct{
		gossip [2]clusterMsgDataGossip
	}

	fail struct{
		about clusterMsgDataFail
	}

	publish struct{
		msg clusterMsgDataPublish
	}

	/* UPDATE */
	update struct {
		nodecfg clusterMsgDataUpdate
	}

	/* MODULE */
	module struct {
		msg clusterMsgModule
	}

}
type clusterMsgDataGossip struct {
	nodename string
	ping_sent uint32
	pong_received uint32
	ip string
	port uint16
	cport uint16
	flags uint16
	pport uint16
	notused1 uint16
}
type clusterMsgDataFail struct {
	nodename string
}
type clusterMsgDataPublish struct {
	channel_len uint32
	message_len uint32
	bulk_data [8]uint8 /* 8 bytes just as placeholder. */
}
type clusterMsgDataUpdate struct {
	configEpoch uint64 /* Config epoch of the specified instance. */
	nodename string /* Name of the slots owner. */
	slots [CLUSTER_SLOTS/8]uint8 /* Slots bitmap. */
}
type clusterMsgModule struct {
	module_id uint64
	len uint32
	ttype uint8
	bulk_data [3]uint8
}

const (
	NET_IP_STR_LEN = 46
)

type clusterNode struct {
	ctime mstime_t //创建结点的时间
	name [CLUSTER_NAMELEN]byte//结点名字，由40个16进制字符组成
	flags int //see CLUSTER_NODE_MASTER
	configEpoch int//结点当前纪元，用于实现故障转移
	slots [CLUSTER_SLOTS/8]byte /* 本节点指派的槽，slots是二进制位数组，包含16384个位即2048个字节*/
	slot_info_pairs int
	numslots int//本节点负责处理槽的个数
	ip [NET_IP_STR_LEN]byte//结点ip地址
	port int
	link *clusterLink


}

type clusterLink struct {
	ctime mstime_t//连接创建时间
	fd int//TCP fd
	sndbuf sds //输出缓冲区，保存着发给其他节点的数据
	rcvbuf sds //输入缓冲区
	node *clusterNode//与这个连接相关联的结点，没有就nil
}

type clusterState struct {//当前结点视角下，集群目前所处的状态
	myself *clusterNode//指向当前node
	currentEpoch uint64//集群的纪元
	state int//集群当前状态
	size int//结点数量（有槽的才算）
	nodes *dict//集群结点名单（包含myself），key=>结点名字，val=>对应的 clusterNode
	slots *[CLUSTER_SLOTS]clusterNode
}