package src

import (
	"net"
)

const (
	C_OK = 0
	C_ERR = -1
)
/*
typedef struct ConnectionType {
    void (*ae_handler)(struct aeEventLoop *el, int fd, void *clientData, int mask);
    int (*connect)(struct connection *conn, const char *addr, int port, const char *source_addr, ConnectionCallbackFunc connect_handler);
    int (*write)(struct connection *conn, const void *data, size_t data_len);
    int (*writev)(struct connection *conn, const struct iovec *iov, int iovcnt);
    int (*read)(struct connection *conn, void *buf, size_t buf_len);
    void (*close)(struct connection *conn);
    int (*accept)(struct connection *conn, ConnectionCallbackFunc accept_handler);
    int (*set_write_handler)(struct connection *conn, ConnectionCallbackFunc handler, int barrier);
    int (*set_read_handler)(struct connection *conn, ConnectionCallbackFunc handler);
    const char *(*get_last_error)(struct connection *conn);
    int (*blocking_connect)(struct connection *conn, const char *addr, int port, long long timeout);
    ssize_t (*sync_write)(struct connection *conn, char *ptr, ssize_t size, long long timeout);
    ssize_t (*sync_read)(struct connection *conn, char *ptr, ssize_t size, long long timeout);
    ssize_t (*sync_readline)(struct connection *conn, char *ptr, ssize_t size, long long timeout);
    int (*get_type)(struct connection *conn);
} ConnectionType;

struct connection {
    ConnectionType *type;
    ConnectionState state;
    short int flags;
    short int refs;
    int last_errno;
    void *private_data;
    ConnectionCallbackFunc conn_handler;
    ConnectionCallbackFunc write_handler;
    ConnectionCallbackFunc read_handler;
    int fd;
};
*/

type ConnectionCallbackFunc func(conn *connection)
type size_t = uint
type connectionType struct {
	aeHandler func(el *aeEventLoop, fd int, clientData uintptr, mask int)
	connect func(conn *connection, addr string, port int, sourceAddr string, connectHandler ConnectionCallbackFunc) int
	write func(conn *connection, data uintptr, dataLen size_t) int

	read func(conn *connection, buf net.Buffers, buf_len size_t) int

}
type connection struct {
	ctype *connectionType
	state connectionState
	flags int8
	refs int8
	lastErrno int
	privateData uintptr
	connHandler ConnectionCallbackFunc
	writeHandler ConnectionCallbackFunc
	readHandler ConnectionCallbackFunc
	fd int


	GoConn
}

type GoConn struct {
	goConn net.Conn
}

/* Get the associated private data pointer */
func connGetPrivateData(conn *connection) uintptr {
	return conn.privateData
}

func connRead(conn *connection, buf net.Buffers, buf_len size_t) int {
	return conn.ctype.read(conn, buf, buf_len)
}

func connGetState(conn *connection) connectionState {
	return conn.state
}
func connSocketRead(conn *connection, buf uintptr, buf_len size_t) int {
	ret, errno := read(conn.fd, buf, buf_len, conn)
	if ret == 0 {
		conn.state = CONN_STATE_CLOSED
	} else if ret < 0 && errno != EAGAIN {
		conn.lastErrno = errno;

		/* Don't overwrite the state of a connection that is not already
		 * connected, not to mess with handler callbacks.
		 */
		if (errno != EINTR && conn->state == CONN_STATE_CONNECTED)
			conn->state = CONN_STATE_ERROR;
	}

	return ret;
}

func read(fd int, buf uintptr, bufLen size_t, conn *connection) (int, error) {
	buffer := make([]byte, bufLen)
	n, err := conn.GoConn.goConn.Read(buffer)
	if err != nil {
		return -1, err
	}

	return n, nil
}