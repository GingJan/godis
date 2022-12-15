package src

type ConnectionType struct {
	get_type func(conn *connection)
}
