package src

func listFirst(l *list) *listNode {
	return l.head
}
func listAddNodeHead(list *list, value interface{}) *list {
	node := &listNode{
		prev: nil,
		next: nil,
		val:  value,
	}

	if list.len == 0 {
		list.tail = node
		list.head = list.tail
	} else {
		node.next = list.head
		list.head.prev = node
		list.head = node
	}

	list.len++
	return list
}

func listAddNodeTail(list *list, value interface{}) *list {
	node := &listNode{
		prev: nil,
		next: nil,
		val:  value,
	}

	if list.len ==  0 {
		list.tail = node
		list.head = list.tail
	} else {
		node.prev = list.tail
		list.tail.next = node
		list.tail = node
	}

	list.len++
	return list
}