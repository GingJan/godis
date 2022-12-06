package src

type time_t = int64
type mstime_t = int64
/* COMMAND flags */
const (
	CMD_ARG_NONE = 0
	CMD_ARG_OPTIONAL = 1<<0
	CMD_ARG_MULTIPLE = 1<<1
	CMD_ARG_MULTIPLE_TOKEN = 1<<2
)
/* CLIENT */
const (
	//用于 client.flags
	CLIENT_SLAVE = 1<<0   /* 1代表该client是从 */
	CLIENT_MASTER = 1<<1  /* 2代表该client是从*/
	CLIENT_MONITOR = 1<<2 /* 4 该client是从 monitor*/
	CLIENT_MULTI = 1<<3   /* 8 This client is in a MULTI context */
	CLIENT_BLOCKED = 1<<4 /* 16 client正被阻塞操作等待着 如 BRPOP命令 */
	CLIENT_DIRTY_CAS = 1<<5 /* 32 Watched keys modified. EXEC will fail. */
	CLIENT_CLOSE_AFTER_REPLY = 1<<6 /* 64 Close after writing entire reply. */
	CLIENT_UNBLOCKED = 1<<7 /* 128 This client was unblocked and is stored in server.unblocked_clients */
)

const (
 	MAX_KEYS_BUFFER = 256

	PROTO_REPLY_CHUNK_BYTES = 16 * 1024//16K
)


const (
	IO_THREADS_OP_IDLE = 0
	IO_THREADS_OP_READ = 1
	IO_THREADS_OP_WRITE = 2
)
const (
	PROTO_IOBUF_LEN = 1024*16  /* Generic I/O buffer size */
)

const (
	CLUSTER_SLOTS = 16384//槽slot总个数
	CLUSTER_OK = 0            /* Everything looks ok */
	CLUSTER_FAIL = 1          /* The cluster can't work */
	CLUSTER_NAMELEN = 40      /* sha1 hex length */
	CLUSTER_PORT_INCR = 10000 /* Cluster port = baseport + PORT_INCR */

	/* Cluster node flags and macros. */
	CLUSTER_NODE_MASTER = 1     /* The node is a master */
	CLUSTER_NODE_SLAVE = 2      /* The node is a slave */
	CLUSTER_NODE_PFAIL = 4      /* Failure? Need acknowledge */
	CLUSTER_NODE_FAIL = 8       /* The node is believed to be malfunctioning */
	CLUSTER_NODE_MYSELF = 16    /* This node is myself */
	CLUSTER_NODE_HANDSHAKE = 32 /* We have still to exchange the first ping */
	CLUSTER_NODE_NOADDR =   64  /* We don't know the address of this node */
	CLUSTER_NODE_MEET = 128     /* Send a MEET message to this node */
	CLUSTER_NODE_MIGRATE_TO = 256 /* Master eligible for replica migration. */
	CLUSTER_NODE_NOFAILOVER = 512 /* Slave will not try to failover. */
	CLUSTER_NODE_NULL_NAME = "\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000\000"

)