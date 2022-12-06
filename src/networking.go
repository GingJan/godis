package src

import (
	"sync/atomic"
	"unsafe"
)

var (
	ProcessingEventsWhileBlocked int = 0
)

//读事件的handler
func readQueryFromClient(conn *connection) {
	c := (*client)(unsafe.Pointer(connGetPrivateData(conn)))
	var nread, big_arg int = 0, 0
	var qblen, readlen size_t

	if postponeClientRead(c) {//若当前client连接无数据，则本次先不读取
		return
	}

	atomic.AddInt64(&server.stat_total_reads_processed, 1)
	readlen = PROTO_IOBUF_LEN

	if c.reqtype == 2 {

	}

	qblen = sdslen(c.querybuf)
	// master client's querybuf can grow greedy.
	if (c.flags & CLIENT_MASTER) == 0  && ((big_arg != 0) || sdsalloc(c.querybuf) < PROTO_IOBUF_LEN) {
		/* When reading a BIG_ARG we won't be reading more than that one arg
		 * into the query buffer, so we don't need to pre-allocate more than we
		 * need, so using the non-greedy growing. For an initial allocation of
		 * the query buffer, we also don't wanna use the greedy growth, in order
		 * to avoid collision with the RESIZE_THRESHOLD mechanism. */
		c.querybuf = sdsMakeRoomForNonGreedy(c->querybuf, readlen);
	} else {
		c.querybuf = sdsMakeRoomFor(c->querybuf, readlen);

		/* Read as much as possible from the socket to save read(2) system calls. */
		readlen = sdsavail(c->querybuf);
	}

	nread = connRead(c.conn, c->querybuf+qblen, readlen);
	if nread == -1 {
		if connGetState(conn) == CONN_STATE_CONNECTED {
			return
		} else {
			//serverLog(LL_VERBOSE, "Reading from client: %s",connGetLastError(c->conn));
			freeClientAsync(c)
			goto done
		}
	} else if (nread == 0) {
		if (server.verbosity <= LL_VERBOSE) {
			sds info = catClientInfoString(sdsempty(), c)
			serverLog(LL_VERBOSE, "Client closed connection %s", info)
			sdsfree(info)
		}
		freeClientAsync(c)
		goto done
	}

done:
	beforeNextClient(c)
}

//异步关闭client（先放入异步关闭队列）
func freeClientAsync(c *client) {
	if c.flags & CLIENT_CLOSE_ASAP || c.flags & CLIENT_SCRIPT {
		return
	}

	c.flags |= CLIENT_CLOSE_ASAP
	if server.io_threads_num == 1 {
		/* no need to bother with locking if there's just one thread (the main thread) */
		listAddNodeTail(server.clients_to_close,c)
		return
	}

	//下面是线程锁+添加到链尾逻辑
	//static pthread_mutex_t async_free_queue_mutex = PTHREAD_MUTEX_INITIALIZER;
	//pthread_mutex_lock(&async_free_queue_mutex);
	//listAddNodeTail(server.clients_to_close,c);
	//pthread_mutex_unlock(&async_free_queue_mutex);
}

func postponeClientRead(c *client) bool {
	if server.io_threads_active && server.io_threads_do_reads && (ProcessingEventsWhileBlocked == 0) && ((c.flags & (CLIENT_MASTER|CLIENT_SLAVE|CLIENT_BLOCKED)) == 0) && (io_threads_op == IO_THREADS_OP_IDLE) {
		listAddNodeHead(server.clients_pending_read, c)
		c.pending_read_list_node = listFirst(server.clients_pending_read)

		return true
	} else {
		return false
	}
}