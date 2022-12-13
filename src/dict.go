package src

//字典结构
import (
	"fmt"
	"io"
	"math"
	"unsafe"
)

const (
	runningCmd     = 0
	BG_SAVE        = 1
	BG_REWRITE_AOF = 2

	ElasticNone = 0
	ElasticIncr = 1
	ElasticDecr = 2
)

var (
	/* 根据需要，可通过使用 dictEnableResize() / dictDisableResize() 来控制hash table大小变化的开启
		这对Redis来说非常重要，因为可使用copy-on-write，当有子进程执行持久化操作时，
		我们并不想移动太多的内存This is very important

	注意，即使当dict_can_resize设为0，也不意味着全部resize操作都会被禁止，
	当元素个数和bucket个数之间的比率 > dict_force_resize_ratio时，
	hash table依旧可以扩缩容。
	*/
	dict_can_resize bool = true
	dict_force_resize_ratio uint = 5
)

/* -------------------------- hash functions -------------------------------- */
var dict_hash_function_seed [16]uint8_t

func dictSetHashFunctionSeed(seed *[16]uint8_t) {
	copy(dict_hash_function_seed[0:], seed[0:])//TODO 待验证
	//memcpy(unsafe.Pointer(&dict_hash_function_seed), unsafe.Pointer(seed), uint(unsafe.Sizeof(dict_hash_function_seed)))
}

func dictGetHashFunctionSeed() *[16]uint8_t {
	return &dict_hash_function_seed
}

func dictGenHashFunction(key unsafe.Pointer, len size_t) uint64_t {
	return siphash(key, len, &dict_hash_function_seed)
}

func dictGenCaseHashFunction(buf *byte, len size_t) uint64_t {
	return siphash_nocase(buf, len, &dict_hash_function_seed)
}

/* ----------------------------- API 实现 ------------------------- */

/* 重置已由_dictInit()初始化的hash table （参数d）*/
func _dictReset(d *dict, htidx int) {
	d.ht_table[htidx] = nil
	d.ht_size_exp[htidx] = -1
	d.ht_used[htidx] = 0
}

/* 创建一个全新的hash table */
func dictCreate(typei *dictType) *dict {
	var d *dict = &dict{}
	zmalloc(size_t(unsafe.Sizeof(*d)))

	_dictInit(d,typei)
	return d
}

/* 初始化hash table */
func _dictInit(d *dict, typei *dictType) int {
	_dictReset(d, 0)
	_dictReset(d, 1)
	d.typei = typei
	d.rehashidx = -1
	d.pauserehash = 0
	return DICT_OK
}

/* 对旧table进行缩容
 * but with the invariant of a USED/BUCKETS ratio near to <= 1 */
func dictResize(d *dict) int {
	var minimal uint64

	if !dict_can_resize || dictIsRehashing(d) {
		return DICT_ERR
	}

	minimal = d.ht_used[0]
	if minimal < DICT_HT_INITIAL_SIZE {
		minimal = DICT_HT_INITIAL_SIZE
	}

	return dictExpand(d, minimal)
}

/* 扩充或创建hash table
 * 当malloc_failed非空时，空间分配失败时不会panic，而是把malloc_failed置为1
 * 如果执行了扩容则返回 DICT_OK, 如果跳过则返回 DICT_ERR */
func _dictExpand(d *dict, size uint64, malloc_failed *int) int {
	if malloc_failed != nil {
		*malloc_failed = 0
	}

	/* 如果要扩容的空间size小于当前旧table已存key的大小，则该size无效 */
	if dictIsRehashing(d) || d.ht_used[0] > size {
		return DICT_ERR
	}

	/* the new hash table */
	var new_ht_table *[]*dictEntry
	new_ht_size_exp := _dictNextExp(size)

	/* Detect overflows */
	newsize := 1<<new_ht_size_exp
	if newsize < size || newsize * int8(unsafe.Sizeof(&dictEntry{})) < newsize {
		return DICT_ERR
	}

	/* Rehashing to the same table size is not useful. */
	if new_ht_size_exp == d.ht_size_exp[0] {
		return DICT_ERR
	}

	/* 分配新的hash table空间并初始化为nil */
	if malloc_failed != nil {
		//new_ht_table = ztrycalloc(newsize*sizeof(dictEntry*))
		new_ht_table_temp := make([]*dictEntry, newsize)
		new_ht_table = &new_ht_table_temp
		if new_ht_table == nil {
			*malloc_failed = 1
			return DICT_ERR
		}
	} else {
		//new_ht_table = zcalloc(newsize*sizeof(dictEntry*))
		new_ht_table_temp := make([]*dictEntry, newsize)
		new_ht_table = &new_ht_table_temp
	}

	/* 该字典是第一次分配table空间吗，如果是则说明不是在rehash */
	if d.ht_table[0] == nil {
		d.ht_size_exp[0] = new_ht_size_exp
		d.ht_used[0] = 0
		d.ht_table[0] = new_ht_table
		return DICT_OK
	}

	/* 分配第二个hash table以便进行扩容rehash */
	d.ht_size_exp[1] = new_ht_size_exp
	d.ht_used[1] = 0
	d.ht_table[1] = new_ht_table
	d.rehashidx = 0
	return DICT_OK
}

/* 如果扩缩容没有进行，则返回 DICT_ERR */
func dictExpand(d *dict, size uint64) int {
	return _dictExpand(d, size, nil)
}

/* 如果因为内存分配失败导致扩缩容执行失败，则返回 DICT_ERR */
func dictTryExpand(d *dict, size uint64) int {
	var malloc_failed int
	_dictExpand(d, size, &malloc_failed)
	if malloc_failed != 0 {
		return DICT_ERR
	}
	return DICT_OK
}

/* 执行n次渐进式rehash，如果旧table的key还没全部迁移完，则返回true，否则返回false
 *
 * 注意，执行1次渐进式rehash，是指把一个bucket里的全部key（采用拉链法链接hash冲突的key）从旧
 * table移到新table，然而因hash table可能有空的bucket，所以本函数不保证一定会迁移至少一个bucket，
 * 因为本函数执行一次最多只会尝试N*10个空bucket的遍历，如果不加最高尝试次数的限制，可能会导致阻塞很长时间
 */
func dictRehash(d *dict, n int) bool {
	var empty_visits int = n * 10 /* 最大尝试次数 */

	if !dictIsRehashing(d) {
		return false
	}

	for n != 0 && d.ht_used[0] != 0 {
		n-- //剩余的搬移操作次数-1

		/* 注意rehashidx不能溢出，因为ht[0].used !=0 以确保还有更多的元素 Note that rehashidx can't overflow as we are sure there are more
		 * elements because ht[0].used != 0 */
		assert(DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx), "DICTHT_SIZE(d.ht_size_exp[0]) > uint64(d.rehashidx)")

		//找到d.rehashidx对应不为空的bucket
		for (*(d.ht_table[0]))[d.rehashidx] == nil { //todo 需要做d.ht_table[0] != nil 吗
			d.rehashidx++ //继续遍历下一个x，d.ht_table[0][x]
			empty_visits--
			if empty_visits == 0 { //如果 进行了empty_visits次的 空遍历 则先返回
				return true
			}
		}

		de := (*(d.ht_table[0]))[d.rehashidx]
		/* 把全部在旧hashtable的bucket里的key移到新hashtable的bucket里 */
		for de != nil {
			nextde := de.next

			/* 获取key在新hash table的index */
			newHashIdx := d.typei.hashFunction(de.key) & DICTHT_SIZE_MASK(d.ht_size_exp[1])
			de.next = (*(d.ht_table[1]))[newHashIdx]
			(*(d.ht_table[1]))[newHashIdx] = de

			d.ht_used[0]-- //key被迁走，对应used-1
			d.ht_used[1]++ //key被迁入，对应used+1

			de = nextde
		}

		//当前rehashidx的bucket下的全部key都搬移完毕
		(*(d.ht_table[0]))[d.rehashidx] = nil
		//继续下一个rehashidx对应的key的迁移
		d.rehashidx++
	}

	/* 如果已经rehash了整个旧table... */
	if d.ht_used[0] == 0 {
		zfree(unsafe.Pointer(&d.ht_table[0])) //释放旧table的空间
		/* 把新ht_table的指向赋给ht_table[0] */
		d.ht_table[0] = d.ht_table[1]
		d.ht_used[0] = d.ht_used[1]
		d.ht_size_exp[0] = d.ht_size_exp[1]
		//重置ht_table[1]
		_dictReset(d, 1)

		//标记rehash完毕
		d.rehashidx = -1

		return false //标识rehash完毕
	}

	/* 还要继续rehash操作...（渐进式rehash）*/
	return true
}

//返回当前毫秒时间戳
func timeInMilliseconds() int64 {
	var tv timeval

	gettimeofday(&tv, nil)
	return (tv.Unix()*1000)+(tv.UnixNano()/1e6)
}

/* 在ms毫秒内尽量多地执行rehash */
func dictRehashMilliseconds(d *dict, ms int) int {
	if d.pauserehash > 0 {
		return 0
	}

	start := timeInMilliseconds()//当前毫秒时间戳
	rehashes := 0

	for dictRehash(d,100) {//执行100次渐进式rehash
		rehashes += 100
		if timeInMilliseconds() - start > int64(ms) {
			break
		}
	}

	return rehashes
}

//执行1次渐进式rehash
func _dictRehashStep(d *dict) {
	if d.pauserehash == 0 {
		dictRehash(d, 1)
	}
}

/* 往字典添加一个元素 */
func dictAdd(d *dict, key unsafe.Pointer, val unsafe.Pointer) int {
	entry := dictAddRaw(d,key,nil)

	if entry == nil {
		return DICT_ERR
	}

	dictSetVal(d, entry, val)
	return DICT_OK
}

/* 底层的添加或返回现有entry:
 * 本函数添加entry到字典d并返回该entry（不会对entry的value字段进行设置），这可按照调用者的意愿设置value字段
 *
 * This function is also directly exposed to the user API to be called
 * mainly in order to store non-pointers inside the hash value, example:
 *
 * entry := dictAddRaw(dict, mykey, nil)
 * if entry != nil {
 *    dictSetSignedIntegerVal(entry, 1000)
 * }
 *
 * 返回值:
 * 如果key已存在则返回nil，同时如果传入existing指针不为空，则该key对应的entry会被设置到existing上
 * 如果key是新添加的，则返回新entry以便调用者进行其他操作
 */
func dictAddRaw(d *dict, key unsafe.Pointer, existing **dictEntry) *dictEntry {
	var index int64
	var entry *dictEntry
	var htidx int

	if dictIsRehashing(d) {
		_dictRehashStep(d)
	}

	/* 获取新entry的slot下标，如果该key对应的entry已存在dict里，则index=-1 */
	index = _dictKeyIndex(d, key, dictHashKey(d,key), existing)
	if index == -1 {
		return nil
	}

	/* 分配内存并保存新entry
	 * 在最前端插入元素，在数据库系统里，一般假定最新添加的entry被访问的频率/可能性 最高 */
	htidx = 0
	if dictIsRehashing(d) {//如果正在进行rehash，则往新table里插入新key
		htidx = 1
	}

	var metasize size_t = dictMetadataSize(d)

	//分配内存空间并记录
	entry = new(dictEntry)
	zmalloc(size_t(unsafe.Sizeof(*entry)) + metasize)

	if metasize > 0 {
		memset(dictMetadata(entry), 0, metasize)
	}

	entry.next = (*(d.ht_table[htidx]))[index]
	(*(d.ht_table[htidx]))[index] = entry
	d.ht_used[htidx]++

	/* Set the hash entry fields. */
	dictSetKey(d, entry, key)
	return entry
}

/* 添加 或 覆盖旧值（更新）
 * 添加一个元素，若key已存在，则覆盖旧值
 * 如果key是新的则返回1，若key已存在则 dictReplace() 只执行值更新操作
 */
func dictReplace(d *dict, key, val unsafe.Pointer) int {
	var entry,existing *dictEntry
	var auxentry dictEntry

	/* 尝试添加元素，如果key还不存在，dictAdd会返回成功 */
	entry = dictAddRaw(d,key,&existing)
	if entry != nil {
		dictSetVal(d, entry, val)
		return 1
	}

	/* Set the new value and free the old one. Note that it is important
	 * to do that in this order, as the value may just be exactly the same
	 * as the previous one. In this context, think to reference counting,
	 * you want to increment (set), and then decrement (free), and not the
	 * reverse. */
	auxentry = *existing
	dictSetVal(d, existing, val)
	dictFreeVal(d, &auxentry)
	return 0
}

/* 添加或返回已存在的entry
 * dictAddOrFind() 是 dictAddRaw() 的精简版，它总返回指定key对应的entry即使key
 * 已存在dict里（此时将会返回该已存在的key对应的entry） even if the key already
 *
 * 查阅 dictAddRaw() 获取更多信息 */
func dictAddOrFind(d *dict, key unsafe.Pointer) *dictEntry {
	var entry, existing *dictEntry
	entry = dictAddRaw(d,key,&existing)
	if entry != nil {
		return entry
	}

	return existing
}

/* 删除一个元素，是dictDelete()和dictUnlink()的辅助函数，请查阅这两函数的注释
本函数会先遍历查找key对应的entry，然后再删除之。
*/
func dictGenericDelete(d *dict, key unsafe.Pointer, nofree bool) *dictEntry {
	var h, idx uint64_t
	var he, prevHe *dictEntry
	var table int

	/* dict 是空的 */
	if dictSize(d) == 0 {
		return nil
	}

	if dictIsRehashing(d) {
		_dictRehashStep(d) //进行rehash操作
	}

	h = dictHashKey(d, key) //计算key的hash值
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		he = (*(d.ht_table[table]))[idx]
		prevHe = nil
		for he != nil {
			if key == he.key || dictCompareKeys(d, key, he.key) {
				/* 从list里unlink该key（对应的entry） */
				if prevHe != nil {
					prevHe.next = he.next
				} else {
					(*(d.ht_table[table]))[idx] = he.next
				}

				if !nofree { //需要释放被删元素的占用空间
					dictFreeUnlinkedEntry(d, he) //释放he占的空间
				}

				d.ht_used[table]-- //已用个数-1
				return he
			}

			prevHe = he //bucket里的拉链的第一个entry并不是要删除的key，遍历到下一个key进行
			he = he.next
		}

		if !dictIsRehashing(d) {
			break
		}
	}

	return nil /* not found */
}

/* 从ht里删除并释放 key，成功返回DICT_OK，元素找不到则返回 DICT_ERR
本函数会先查找key对应的entry，然后再删除之。
*/
func dictDelete(ht *dict, key unsafe.Pointer) int {
	if dictGenericDelete(ht, key, false) != nil { //dictGenericDelete函数会先遍历查找key对应的entry，然后再删除之。
		return DICT_OK
	}

	return DICT_ERR
}

/* 从ht里删除key，但不释放key的空间 */
func dictUnlink(d *dict, key unsafe.Pointer) *dictEntry {
	return dictGenericDelete(d, key, true)
}

/* 调用dictUnlink()后，调用本函数才真正地释放he占的空间，当he为nil时，调用该函数也是安全的 */
func dictFreeUnlinkedEntry(d *dict, he *dictEntry) {
	if he == nil {
		return
	}

	dictFreeKey(d, he)        //释放he对应的key
	dictFreeVal(d, he)        //释放he对应的val
	zfree(unsafe.Pointer(he)) //把he释放
}

/* 摧毁一整个dict字典，htidx只可传0（旧table）或1（新table） */
func _dictClear(d *dict, htidx int, callback func(*dict)) int {
	var i uint64

	/* 释放全部元素 */
	for i = 0; i < DICTHT_SIZE(d.ht_size_exp[htidx]) && d.ht_used[htidx] > 0; i++ {
		var he, nextHe *dictEntry

		if callback != nil && (i&65535) == 0 {
			callback(d)
		}

		he = (*(d.ht_table[htidx]))[i]
		if he == nil {
			continue
		}

		for he != nil {
			nextHe = he.next
			dictFreeKey(d, he)
			dictFreeVal(d, he)
			zfree(unsafe.Pointer(he))
			d.ht_used[htidx]--
			he = nextHe
		}
	}

	/* 释放table的空间和分配的缓存结构 */
	zfree(unsafe.Pointer(&d.ht_table[htidx]))

	_dictReset(d, htidx)

	return DICT_OK
}

/* 清除并释放hash table */
func dictRelease(d *dict) {
	_dictClear(d, 0, nil)
	_dictClear(d, 1, nil)
	zfree(unsafe.Pointer(d))
}

//寻找key对应的entry
func dictFind(d *dict, key unsafe.Pointer) *dictEntry {
	var he *dictEntry
	var h, idx, table uint64_t

	if dictSize(d) == 0 { /* 字典是空的 */
		return nil
	}

	if dictIsRehashing(d) {
		_dictRehashStep(d) //执行渐进式rehash
	}

	h = dictHashKey(d, key) //计算key的hash值
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		he = (*(d.ht_table[table]))[idx]
		for he != nil {
			if key == he.key || dictCompareKeys(d, key, he.key) { //是要找的key
				return he //返回key对应的entry
			}

			he = he.next //继续遍历拉链里的下一个entry
		}

		if !dictIsRehashing(d) {
			return nil
		}
	}

	return nil
}

//获取key对应的val
func dictFetchValue(d *dict, key unsafe.Pointer) unsafe.Pointer {
	he := dictFind(d, key)
	if he != nil {
		return dictGetVal(he)
	}
	return nil
}

/* 返回的64位fingerprint指纹，表示dict字典在指定时间的状态，由几个dict的字段通过 异或 运算得出，
 * 当初始化一个非安全的iterator时，会获取dict字典的指纹fingerprint，在iterator释放时会再次检查指纹是否一致
 * 如果不一致则说明iterator的调用者在字典遍历期间执行了禁止的操作（因而导致字典指纹发生了变化）。
 */
func dictFingerprint(d *dict) uint64 {
	var integers [6]uint64
	var j int

	integers[0] = uint64(uintptr(unsafe.Pointer(d.ht_table[0])))
	integers[1] = uint64(d.ht_size_exp[0])
	integers[2] = d.ht_used[0]
	integers[3] = uint64(uintptr(unsafe.Pointer(d.ht_table[1])))
	integers[4] = uint64(d.ht_size_exp[1])
	integers[5] = d.ht_used[1]

	/* 使用类似以下公式计算最后的hash值:
	 * Result = hash(hash(hash(int1)+int2)+int3) ...
	 * 几个hash的顺序不同，得出的最终hash值也不同
	 */
	var hash uint64 = 0
	for j = 0; j < 6; j++ {
		hash += integers[j]
		/* For the hashing step we use Tomas Wang's 64 bit integer hash. */
		hash = (^hash) + (hash << 21) // hash = (hash << 21) - hash - 1;
		hash = hash ^ (hash >> 24)
		hash = (hash + (hash << 3)) + (hash << 8) // hash * 265
		hash = hash ^ (hash >> 14)
		hash = (hash + (hash << 2)) + (hash << 4) // hash * 21
		hash = hash ^ (hash >> 28)
		hash = hash + (hash << 31)
	}
	return hash
}

func dictGetIterator(d *dict) *dictIterator {
	iter := dictIterator{}
	dictInitIterator(&iter, d)
	return &iter
}

func dictGetSafeIterator(d *dict) *dictIterator {
	i := dictGetIterator(d)
	i.safe = true
	return i
}

func dictNext(iter *dictIterator) *dictEntry {
	for {
		if iter.entry == nil {//iterator第一次迭代
			if iter.index == -1 && iter.table == 0 {//第一次迭代
				if iter.safe {//当iterator设为安全模式时，则需要先暂停字典的rehash 再遍历
					dictPauseRehashing(iter.d)
				} else {
					iter.fingerprint = dictFingerprint(iter.d)//获取当前字典的指纹
				}
			}

			iter.index++
			if iter.index >= int64(DICTHT_SIZE(iter.d.ht_size_exp[iter.table])) {//如果在迭代时，字典的大小发生变化，说明可能正在rehash。否则就是遍历完毕
				if dictIsRehashing(iter.d) && iter.table == 0 {//字典正在rehash
					iter.table++//则去迭代新的table
					iter.index = 0//从新table的0开始迭代
				} else {
					break//字典遍历完毕
				}
			}

			iter.entry = (*(iter.d.ht_table[iter.table]))[iter.index]//新table的entry
		} else {
			iter.entry = iter.nextEntry//指向到原table的下一个entry
		}

		if iter.entry != nil {
			/* 保存entry指向的下一个entry，因为返回的entry可能会被调用者删除 */
			iter.nextEntry = iter.entry.next
			return iter.entry
		}
	}

	return nil
}

//释放iterator
func dictReleaseIterator(iter *dictIterator) {
	dictResetIterator(iter)
	zfree(unsafe.Pointer(iter))
}

/* 从hash table里随机返回一个entry，本函数可用于实现随机化算法，基本过程是从新或旧table里随机取一个bucket
再从该bucket内的entry链表里随机取一个entry返回
*/
func dictGetRandomKey(d *dict) *dictEntry{
	if dictSize(d) == 0 {//字典里无数据
		return nil
	}

	if dictIsRehashing(d) {//当前字典d正进行rehash
		_dictRehashStep(d)//执行一次rehash
	}

	var h uint64
	var he *dictEntry
	if dictIsRehashing(d) {//若字典当前正在进行rehash
		s0 := DICTHT_SIZE(d.ht_size_exp[0])//获取字典的bucket个数
	do1:
		/* 可以确定的是从index=0到rehashidx-1内都没有元素 */
		h = uint64(d.rehashidx + int64(randomULong()) % (int64(dictSlots(d)) - d.rehashidx))
		if h > s0 {
			he = (*(d.ht_table[1]))[h - s0]//获取最后一个bucket
		} else {
			he = (*(d.ht_table[0]))[h]
		}
		if he == nil {
			goto do1//如果随机取的bucket里没有entry，则重新再随机取bucket
		}
	} else {//字典当前没在进行rehash
		mask := DICTHT_SIZE_MASK(d.ht_size_exp[0])
	do2:
		//也是随机取一个bucket
		h = randomULong() & mask
		he = (*(d.ht_table[0]))[h]

		if he == nil {
			goto do2
		}
	}

	/* 至此找到非空的bucket，但它是一个链表，现需要从链表里再随机取一个元素。
	唯一比较好的方式就是计算元素个数，然后随机取个index下标
	*/
	listlen := 0//bucket里 链表的长度（即entry的个数）
	orighe := he
	for he != nil {
		he = he.next
		listlen++
	}
	listele := random() % listlen
	he = orighe
	for listele != 0 {
		listele--
		he = he.next
	}

	return he//bucket里的链表的随机一个entry
}

/* 本函数对字典dict进行采样，从随机位置返回几个key
 *
 * 它不保证指定数量count个key，也不保证返回的元素不会重复，但是会尽量保证返回count个key且尽量不重复
 *
 * Returned pointers to hash table entries are stored into 'des' that
 * points to an array of dictEntry pointers. The array must have room for
 * at least 'count' elements, that is the argument we pass to the function
 * to tell how many random elements we need.
 *
 * The function returns the number of items stored into 'des', that may
 * be less than 'count' if the hash table has less than 'count' elements
 * inside, or if not enough elements were found in a reasonable amount of
 * steps.
 *
 * Note that this function is not suitable when you need a good distribution
 * of the returned items, but only when you need to "sample" a given number
 * of continuous elements to run some kind of algorithm or to produce
 * statistics. However the function is much faster than dictGetRandomKey()
 * at producing N elements. */
func dictGetSomeKeys(d *dict, des *[]*dictEntry, count uint) uint {
	var j uint /* 内部迭代时的新旧table下标 0 or 1. */

	if uint(dictSize(d)) < count {
		count = uint(dictSize(d))
	}

	maxsteps := uint64(count) * 10//最大尝试次数

	/* 获取count个key就做count次rehash */
	for j = 0; j < count; j++ {
		if dictIsRehashing(d) {
			_dictRehashStep(d)
		} else {
			break
		}
	}

	var tables uint /* =0或1 */
	if dictIsRehashing(d) {//正在进行rehash操作
		tables = 1
	} else {
		tables = 0
	}

	maxsizemask := DICTHT_SIZE_MASK(d.ht_size_exp[0])
	if tables > 0 && maxsizemask < DICTHT_SIZE_MASK(d.ht_size_exp[1]) {//如果tables > 0说明正在进行rehash，所以maxsizemask取新table的值
		maxsizemask = DICTHT_SIZE_MASK(d.ht_size_exp[1])
	}

	i := randomULong() & maxsizemask/* 从任意位置开始遍历 */
	var emptylen uint64 = 0 /* 到目前为止连续空entry的长度 */

	var stored uint = 0//已收集的dictEntry个数
	for stored < count && maxsteps != 0 {
		maxsteps--

		for j = 0; j <= tables; j++ {//tables=1代表正在进行rehash
			/* Invariant of the dict.c rehashing: 取决于rehash期间，已在ht[0]已访问过的下标。
			没有已填充的bucket，所以可以跳过ht[0]上从0到idx-1的下标 */
			if tables == 1 && j == 0 && i < uint64(d.rehashidx) {//tables=1代表正在进行rehash，j=0表示本函数当前进行对旧table进行遍历
				/* 再者，如果现在i已超过新table的大小，则意味新旧两个表对应的下标i都不存在元素了。
				这种情况可以跳过（这一般是发生在字典缩容时）*/
				if i >= DICTHT_SIZE(d.ht_size_exp[1]) {//正在缩容
					i = uint64(d.rehashidx)
				} else {
					continue//跳过ht[0]上从0到idx-1的下标
				}
			}

			if i >= DICTHT_SIZE(d.ht_size_exp[j]) {//本函数已对本table遍历完毕
				continue
			}

			/* 超过本table=j的范围了 */
			var he *dictEntry = (*(d.ht_table[j]))[i]

			/* 计算连续为空的bucket个数，如果连续空bucket的个数达到「count」个且至少5个，就跳到其他位置（更新i） */
			if he == nil {
				emptylen++
				if emptylen >= 5 && emptylen > uint64(count) {
					i = randomULong() & maxsizemask
					emptylen = 0
				}
			} else {
				emptylen = 0
				desI := 0
				for he != nil {
					/* 收集迭代时发现的非空bucket中的所有元素 */
					(*des)[desI] = he//*des = he
					desI++//des++

					he = he.next
					stored++
					if stored == count {
						return stored
					}
				}
			}
		}

		i = (i+1) & maxsizemask
	}

	return stored
}

/* 本函数类似dictGetRandomKey()，但会做更多处理以确保返回的元素在分布上更科学
 *
 * This function improves the distribution because the dictGetRandomKey()
 * problem is that it selects a random bucket, then it selects a random
 * element from the chain in the bucket. However elements being in different
 * chain lengths will have different probabilities of being reported. With
 * this function instead what we do is to consider a "linear" range of the table
 * that may be constituted of N buckets with chains of different lengths
 * appearing one after the other. Then we report a random element in the range.
 * In this way we smooth away the problem of different chain lengths. */
const GETFAIR_NUM_ENTRIES = 15
func dictGetFairRandomKey(d *dict) *dictEntry {
	var entries []*dictEntry = make([]*dictEntry, GETFAIR_NUM_ENTRIES)
	var count uint = dictGetSomeKeys(d, &entries, GETFAIR_NUM_ENTRIES)
	/* Note that dictGetSomeKeys() may return zero elements in an unlucky
	 * run() even if there are actually elements inside the hash table. So
	 * when we get zero, we call the true dictGetRandomKey() that will always
	 * yield the element if the hash table has at least one. */
	if count == 0 {
		return dictGetRandomKey(d)
	}

	var idx uint = uint(random()) % count
	return entries[idx]
}

/* 按位反转，参考算法: http://graphics.stanford.edu/~seander/bithacks.html#ReverseParallel */
func rev(v uint64) uint64 {
	s := CHAR_BIT * unsafe.Sizeof(v) // bit size; must be power of 2
	var mask uint64 = ^0
	s = s >> 1
	for s > 0 {
		mask ^= (mask << s)
		v = ((v >> s) & mask) | ((v << s) & ^mask)
		s = s >> 1
	}
	return v
}

/* dictScan() 用于遍历字典的元素
 *
 * Iterating works the following way:
 *
 * 1）1) Initially you call the function using a cursor (v) value of 0.
 * 2) The function performs one step of the iteration, and returns the
 *    new cursor value you must use in the next call.
 * 3) When the returned cursor is 0, the iteration is complete.
 *
 * The function guarantees all elements present in the
 * dictionary get returned between the start and end of the iteration.
 * However it is possible some elements get returned multiple times.
 *
 * For every element returned, the callback argument 'fn' is
 * called with 'privdata' as first argument and the dictionary entry
 * 'de' as second argument.
 *
 * HOW IT WORKS.
 *
 * The iteration algorithm was designed by Pieter Noordhuis.
 * The main idea is to increment a cursor starting from the higher order
 * bits. That is, instead of incrementing the cursor normally, the bits
 * of the cursor are reversed, then the cursor is incremented, and finally
 * the bits are reversed again.
 *
 * This strategy is needed because the hash table may be resized between
 * iteration calls.
 *
 * dict.c hash tables are always power of two in size, and they
 * use chaining, so the position of an element in a given table is given
 * by computing the bitwise AND between Hash(key) and SIZE-1
 * (where SIZE-1 is always the mask that is equivalent to taking the rest
 *  of the division between the Hash of the key and SIZE).
 *
 * For example if the current hash table size is 16, the mask is
 * (in binary) 1111. The position of a key in the hash table will always be
 * the last four bits of the hash output, and so forth.
 *
 * WHAT HAPPENS IF THE TABLE CHANGES IN SIZE?
 *
 * If the hash table grows, elements can go anywhere in one multiple of
 * the old bucket: for example let's say we already iterated with
 * a 4 bit cursor 1100 (the mask is 1111 because hash table size = 16).
 *
 * If the hash table will be resized to 64 elements, then the new mask will
 * be 111111. The new buckets you obtain by substituting in ??1100
 * with either 0 or 1 can be targeted only by keys we already visited
 * when scanning the bucket 1100 in the smaller hash table.
 *
 * By iterating the higher bits first, because of the inverted counter, the
 * cursor does not need to restart if the table size gets bigger. It will
 * continue iterating using cursors without '1100' at the end, and also
 * without any other combination of the final 4 bits already explored.
 *
 * Similarly when the table size shrinks over time, for example going from
 * 16 to 8, if a combination of the lower three bits (the mask for size 8
 * is 111) were already completely explored, it would not be visited again
 * because we are sure we tried, for example, both 0111 and 1111 (all the
 * variations of the higher bit) so we don't need to test it again.
 *
 * WAIT... YOU HAVE *TWO* TABLES DURING REHASHING!
 *
 * Yes, this is true, but we always iterate the smaller table first, then
 * we test all the expansions of the current cursor into the larger
 * table. For example if the current cursor is 101 and we also have a
 * larger table of size 16, we also test (0)101 and (1)101 inside the larger
 * table. This reduces the problem back to having only one table, where
 * the larger one, if it exists, is just an expansion of the smaller one.
 *
 * LIMITATIONS
 *
 * This iterator is completely stateless, and this is a huge advantage,
 * including no additional memory used.
 *
 * The disadvantages resulting from this design are:
 *
 * 1) It is possible we return elements more than once. However this is usually
 *    easy to deal with in the application level.
 * 2) The iterator must return multiple elements per call, as it needs to always
 *    return all the keys chained in a given bucket, and all the expansions, so
 *    we are sure we don't miss keys moving during rehashing.
 * 3) The reverse cursor is somewhat hard to understand at first, but this
 *    comment is supposed to help.
 */
func dictScan(d *dict, v uint64, fn dictScanFunction, bucketfn dictScanBucketFunction, privdata unsafe.Pointer) uint64 {
	var de, next *dictEntry
	var m1 uint64

	if dictSize(d) == 0 {
		return 0
	}

	/* This is needed in case the scan callback tries to do dictFind or alike. */
	dictPauseRehashing(d)

	if !dictIsRehashing(d) {//字典当前没在进行rehash
		htidx0 := 0
		mask0 := DICTHT_SIZE_MASK(d.ht_size_exp[htidx0])

		/* Emit entries at cursor */
		if bucketfn != nil  {
			bucketfn(d, &(*(d.ht_table[htidx0]))[v & mask0])
		}

		de = (*(d.ht_table[htidx0]))[v & mask0]
		for de != nil {
			next = de.next
			fn(privdata, de)
			de = next
		}

		/* Set unmasked bits so incrementing the reversed cursor
		 * operates on the masked bits */
		v |= ^mask0

		/* Increment the reverse cursor */
		v = rev(v)
		v++
		v = rev(v)
	} else {
		htidx0 := 0
		htidx1 := 1

		/* Make sure t0 is the smaller and t1 is the bigger table */
		if DICTHT_SIZE(d.ht_size_exp[htidx0]) > DICTHT_SIZE(d.ht_size_exp[htidx1]) {
			htidx0 = 1
			htidx1 = 0
		}

		mask0 := DICTHT_SIZE_MASK(d.ht_size_exp[htidx0])
		mask1 := DICTHT_SIZE_MASK(d.ht_size_exp[htidx1])

		/* Emit entries at cursor */
		if bucketfn != nil {
			bucketfn(d, &(*(d.ht_table[htidx0]))[v & mask0])
		}

		de = (*(d.ht_table[htidx0]))[v & mask0]
		for de != nil {
			next = de.next
			fn(privdata, de)
			de = next
		}

		/* Iterate over indices in larger table that are the expansion
		 * of the index pointed to by the cursor in the smaller table */
	do: {
		/* Emit entries at cursor */
		if bucketfn != nil {
			bucketfn(d, &(*(d.ht_table[htidx1]))[v & m1])
		}

		de = (*(d.ht_table[htidx1]))[v & m1]
		for de != nil {
			next = de.next
			fn(privdata, de)
			de = next
		}

		/* Increment the reverse cursor not covered by the smaller mask.*/
		v |= ^m1
		v = rev(v)
		v++
		v = rev(v)

		/* Continue while bits covered by mask difference is non-zero */
		if v & (mask0 ^ mask1) != 0 {
			goto do
		}
	}
	}

	dictResumeRehashing(d)

	return v
}

/* ------------------------- 私有函数 ------------------------------ */

/* Because we may need to allocate huge memory chunk at once when dict
 * expands, we will check this allocation is allowed or not if the dict
 * type has expandAllowed member function. */
func dictTypeExpandAllowed(d *dict) int {
	if d.typei.expandAllowed == nil {
		return 1
	}
	return d.typei.expandAllowed(
		size_t(DICTHT_SIZE(_dictNextExp(d.ht_used[0] + 1)) * uint64(unsafe.Sizeof(&dictEntry{}))),
		float64(d.ht_used[0] / DICTHT_SIZE(d.ht_size_exp[0])),
	)
}

/* Expand the hash table if needed */
func _dictExpandIfNeeded(d *dict) int {
	/* Incremental rehashing already in progress. Return. */
	if dictIsRehashing(d) {
		return DICT_OK
	}

	/* If the hash table is empty expand it to the initial size. */
	if DICTHT_SIZE(d.ht_size_exp[0]) == 0 {
		return dictExpand(d, DICT_HT_INITIAL_SIZE)
	}

	/* If we reached the 1:1 ratio, and we are allowed to resize the hash
	 * table (global setting) or we should avoid it but the ratio between
	 * elements/buckets is over the "safe" threshold, we resize doubling
	 * the number of buckets. */
	if (d.ht_used[0] >= DICTHT_SIZE(d.ht_size_exp[0])) &&
		(dict_can_resize || ((d.ht_used[0] / DICTHT_SIZE(d.ht_size_exp[0])) > uint64(dict_force_resize_ratio))) &&
		(dictTypeExpandAllowed(d) != 0) {
		return dictExpand(d, d.ht_used[0] + 1)
	}

	return DICT_OK
}

/* TODO: clz optimization */
/* Our hash table capability is a power of two */
func _dictNextExp(size uint64) int8 {
	e := DICT_HT_INITIAL_EXP

	if size >= uint64(LONG_MAX) {
		return int8(8*unsafe.Sizeof(lONG)-1)
	}

	for {
		if (1<<e) >= size {
			return int8(e)
		}

		e++
	}
}

/* 返回可存放key对应entry的slot下标
 * 若果key对应entry已存在则返回-1，并且existing参数会被置为该entry
 *
 * 注意，如果hash table正在进行rehash中，返回的slot下标是新table的下标 */
func _dictKeyIndex(d *dict, key unsafe.Pointer, hash uint64_t, existing **dictEntry) int64 {
	var idx, table uint64
	var he *dictEntry
	if existing != nil {
		*existing = nil
	}

	/* 如果需要，则扩容hash table */
	if _dictExpandIfNeeded(d) == DICT_ERR {
		return -1
	}

	for table = 0; table <= 1; table++ {//对新旧两站table逐一进行遍历
		idx = hash & DICTHT_SIZE_MASK(d.ht_size_exp[table])//用hash值和hash mask掩码计算出对应下标
		// 若该idx上已有entry，则遍历该entry（链）
		he = (*(d.ht_table[table]))[idx]
		for he != nil {//遍历slot对应的entry链，
			if key == he.key || dictCompareKeys(d, key, he.key) {//直到找到指定key
				if existing != nil {
					*existing = he//返回指定key（已存在）对应entry
				}

				return -1//该key已存在
			}

			he = he.next
		}

		if !dictIsRehashing(d) {//如果没有在进行rehash，则不用扫描新table
			break
		}
	}

	return int64(idx)//该idx还没有entry
}

func dictEmpty(d *dict, callback func(*dict)) {
	_dictClear(d,0, callback)
	_dictClear(d,1, callback)
	d.rehashidx = -1
	d.pauserehash = 0
}

func dictEnableResize() {
	dict_can_resize = true
}

func dictDisableResize() {
	dict_can_resize = false
}

func dictGetHash(d *dict, key unsafe.Pointer) uint64_t {
	return dictHashKey(d, key)
}

/* Finds the dictEntry reference by using pointer and pre-calculated hash.
 * oldkey is a dead pointer and should not be accessed.
 * the hash value should be provided using dictGetHash.
 * no string / key comparison is performed.
 * return value is the reference to the dictEntry if found, or NULL if not found. */
func dictFindEntryRefByPtrAndHash(d *dict, oldptr unsafe.Pointer, hash uint64_t) **dictEntry {
	var he *dictEntry
	var heref **dictEntry
	var idx, table uint64

	if dictSize(d) == 0 { /* dict is empty */
		return nil
	}

	for table = 0; table <= 1; table++ {
		idx = hash & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		heref = &(*(d.ht_table[table]))[idx]
		he = *heref
		for he != nil {
			if oldptr==he.key {
				return heref
			}

			heref = &he.next
			he = *heref
		}

		if !dictIsRehashing(d) {
			return nil
		}
	}
	return nil
}

func (d *dict) getHashIdx(key string) (int, uint64) {
	hash := d.dtype.hashFunction(key)
	htNum := 0
	if d.rehashidx > -1 { //正在进行rehash，所以ht[0]只减不增
		htNum = 1
	}

	return htNum, uint64(hash) & d.ht[htNum].sizemask
}

func (d *dict) addEntry(htNum int, idx uint64, dictEntry *dictEntry) {
	if d.ht[htNum] == nil {
		panic("warning fatal error occurred!")
	}

	if d.ht[htNum].table[idx] != nil {
		dictEntry.next = d.ht[htNum].table[idx]
	}
	d.ht[htNum].table[idx] = dictEntry
	d.ht[htNum].used++
}
func (d *dict) delEntry(htNum int, idx uint64, key string) {
	curr := d.ht[htNum].table[idx]
	for {
		if curr == nil {
			return
		}
		if curr.key == key {
			d.ht[htNum].table[idx] = curr.next
			d.ht[htNum].used--
			//delete(curr)//释放curr
			return
		}
		curr = curr.next
	}
}
func (d *dict) getEntry(htNum int, idx uint64, key string) *dictEntry {
	curr := d.ht[htNum].table[idx]
	for {
		if curr == nil {
			return nil
		}
		if curr.key == key {
			return curr
		}

		curr = curr.next
	}
}
func (d *dict) updateEntry(htNum int, idx uint64, key string, val interface{}) bool {
	entry := d.getEntry(htNum, idx, key)
	if entry == nil {
		return false
	}

	entry.val = val
	return true
}

//扩容
func (d *dict) checkElastic() {
	switch d.needElastic() {
	case ElasticIncr:
		d.doSizeIncr()
	case ElasticDecr:
		d.doSizeDecr()
	}
}
func (d *dict) doSizeIncr() {
	if d.rehashidx == -1 { //本轮第一次扩容
		d.rehashidx = 0
		incrToSize := uint64(math.Pow(float64(2), math.Log2(float64(d.ht[0].used)*2)+1)) //2^n次方
		d.ht[1] = &dictht{
			table:    make([]*dictEntry, 0, incrToSize),
			size:     incrToSize,
			sizemask: incrToSize - 1,
			used:     0,
		}
	}

	for {
		curr := d.ht[0].table[d.rehashidx]
		if curr == nil {
			d.rehashidx++
			if d.rehashidx >= len(d.ht[0].table) { //完成扩容
				d.rehashidx = -1
				d.ht[0] = d.ht[1]
				d.ht[1] = nil
				return
			}

			continue
		} else {
			_, newIdx := d.getHashIdx(curr.key)
			d.ht[1].table[newIdx] = curr
			d.ht[1].used++
			d.ht[0].table[d.rehashidx] = curr.next //移动到下一个
			//delete(curr)//释放当前curr
			d.ht[0].used--

			return //每次操作只进行一次弹性扩容/缩小
		}
	}
}
func (d *dict) doSizeDecr() {}
func (d *dict) needElastic() int8 {
	loadFactor := float64(d.ht[0].used) / float64(d.ht[0].size)
	switch runningCmd {
	case BG_SAVE, BG_REWRITE_AOF:
		return d.elasticThreshold(loadFactor, 5, 0.1)
	default:
		return d.elasticThreshold(loadFactor, 1, 0.1)
	}
}
func (d *dict) elasticThreshold(loadFactor, incr, decr float64) int8 {
	switch {
	case loadFactor >= incr:
		return ElasticIncr
	case loadFactor < decr:
		return ElasticDecr
	default:
		return ElasticNone
	}
}


/* 从ht里查找元素，若找到元素则返回对应entry，接着调用者应调用`dictTwoPhaseUnlinkFree`函数以unlink和释放该entry。
若找不到该元素则返回nil。plink存放的是对应entry的指针。
本函数和`dictTwoPhaseUnlinkFree`应一起成对调用，`dictTwoPhaseUnlinkFind`暂停rehash而`dictTwoPhaseUnlinkFree`恢复rehash

可按照以下方式使用：
var de *dictEntry = dictTwoPhaseUnlinkFind(db.dict,key.ptr,&plink, &table)
//其他代码（但不能修改dict）
dictTwoPhaseUnlinkFree(db.dict,de,plink,table); // 不需要再去找一次
这两函数配合起来，就是先找到要删元素，使用元素，然后删除元素

如果想要再删除某个entry前先查找出来，可按以上示例使用，这样避免先dictFind()再dictDelete()（因为这两函数都会有寻找entry的过程，寻找重复做了两次）
*/
func dictTwoPhaseUnlinkFind(d *dict, key unsafe.Pointer, plink ***dictEntry, table_index *int) *dictEntry {
	var h, idx, table uint64_t

	if dictSize(d) == 0 { /* dict is empty */
		return nil
	}

	//进行渐进式rehash
	if dictIsRehashing(d) {
		_dictRehashStep(d)
	}

	h = dictHashKey(d, key)
	//遍历查找key对应entry
	for table = 0; table <= 1; table++ {
		idx = h & DICTHT_SIZE_MASK(d.ht_size_exp[table])
		var ref **dictEntry = &((*(d.ht_table[table]))[idx])
		for *ref != nil {
			if key == (*ref).key || dictCompareKeys(d, key, (*ref).key) {
				*table_index = int(table)
				*plink = ref
				dictPauseRehashing(d) //暂停rehash
				return *ref
			}
			ref = &(*ref).next
		}

		if !dictIsRehashing(d) { //如果不在进行rehash，则无需遍历ht_table[1]了
			return nil
		}
	}

	return nil
}

// 把he从d里移除并释放he的空间
func dictTwoPhaseUnlinkFree(d *dict, he *dictEntry, plink **dictEntry, table_index int) {
	if he == nil {
		return
	}

	d.ht_used[table_index]--
	*plink = he.next

	dictFreeKey(d, he)
	dictFreeVal(d, he)
	zfree(unsafe.Pointer(he))
	dictResumeRehashing(d) //恢复rehash
}


/* ------------------------------- Debugging ---------------------------------*/

const DICT_STATS_VECTLEN = 50
func _dictGetStatsHt(buf *[]byte, bufsize size_t, d *dict, htidx int) size_t {
	var i, chainlen uint64
	var slots, maxchainlen uint64 = 0, 0
	var totchainlen uint64 = 0
	var clvector [DICT_STATS_VECTLEN]uint64
	var l size_t = 0

	if d.ht_used[htidx] == 0 {
		return snprintf(buf, bufsize, "No stats available for empty dictionaries\n")
	}

	/* Compute stats. */
	for i = 0; i < DICT_STATS_VECTLEN; i++ {
		clvector[i] = 0
	}
	for i = 0; i < DICTHT_SIZE(d.ht_size_exp[htidx]) i++ {
		var he *dictEntry

		if (*(d.ht_table[htidx]))[i] == nil {
			clvector[0]++
			continue
		}

		slots++
		/* For each hash entry on this slot... */
		chainlen = 0
		he = (*(d.ht_table[htidx]))[i]
		for he != nil {
			chainlen++
			he = he.next
		}

		if chainlen < DICT_STATS_VECTLEN {
			clvector[chainlen]++
		} else {
			clvector[DICT_STATS_VECTLEN-1]++
		}

		if chainlen > maxchainlen {
			maxchainlen = chainlen
		}
		totchainlen += chainlen
	}

	/* Generate human readable stats. */
	htidxStr := ""
	if htidx == 0 {
		htidxStr = "main hash table"
	} else {
		htidxStr = "rehashing target"
	}

	l += snprintf(buf, bufsize-l,
	`Hash table %d stats (%s):\n
	" table size: %d\n"
	" number of elements: %d\n"
	" different slots: %d\n"
	" max chain length: %d\n"
	" avg chain length (counted): %.02f\n"
	" avg chain length (computed): %.02f\n"
	" Chain length distribution:\n`,
	htidx, htidxStr,
	DICTHT_SIZE(d.ht_size_exp[htidx]),
	d.ht_used[htidx], slots, maxchainlen,
	float32(totchainlen/slots), float32(d.ht_used[htidx]/slots))

	for i = 0; i < DICT_STATS_VECTLEN-1; i++ {
		if clvector[i] == 0 {
			continue
		}
		if l >= bufsize {
			break
		}

		l += snprintf(buf, bufsize-l,
		"   %ld: %ld (%.02f%%)\n",
		i, clvector[i], float32(clvector[i]/DICTHT_SIZE(d.ht_size_exp[htidx]))*100)
	}

	/* Unlike snprintf(), return the number of characters actually written. */
	if bufsize != 0 {
		(*buf)[bufsize-1] = byte('\0')
	}

	return size_t(len(*buf))
}

func dictGetStats(buf *[]byte, bufsize size_t, d *dict) {
	var l size_t
	var orig_buf *[]byte = buf
	var orig_bufsize size_t = bufsize

	l = _dictGetStatsHt(buf,bufsize,d,0)
	buf += l
	bufsize -= l
	if dictIsRehashing(d) && bufsize > 0 {
		_dictGetStatsHt(buf,bufsize,d,1)
	}

	/* Make sure there is a NULL term at the end. */
	if orig_bufsize != 0 {
		orig_buf[orig_bufsize-1] = '\0'
	}
}


//fuck

//初始化iterator
func dictInitIterator(iter *dictIterator, d *dict) {
	iter.d = d
	iter.table = 0
	iter.index = -1
	iter.safe = false
	iter.entry = nil
	iter.nextEntry = nil
}

//初始化安全的iterator
func dictInitSafeIterator(iter *dictIterator, d *dict) {
	dictInitIterator(iter, d) //初始化
	iter.safe = true
}

// 重置iter
func dictResetIterator(iter *dictIterator) {
	if !(iter.index == -1 && iter.table == 0) {
		if iter.safe {//如果iter是安全模式的，则恢复dict的rehash操作
			dictResumeRehashing(iter.d)
		} else {//判断iter开始遍历前和遍历后，dict字典的指纹是否一致，若不一致则报错（说明遍历期间字典被修改了）
			assert(iter.fingerprint == dictFingerprint(iter.d), "iter.fingerprint == dictFingerprint(iter.d)")
		}
	}
}

