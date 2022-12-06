package src

type pubsubPattern struct {
	client *client//订阅的client
	pattern *robj//被订阅的模式
}
func newPubsubPattern() *pubsubPattern {
	return &pubsubPattern{}
}
func (ps *pubsubPattern) psubscribe(client *client, allInputPatterns []string) {
	for _, p := range allInputPatterns {
		ps := newPubsubPattern()

		robj := &redisObject{}
		robj.ptr = p
		ps.pattern = robj
		ps.client = client

		server.pubsubPatterns.append(ps)
	}
}
