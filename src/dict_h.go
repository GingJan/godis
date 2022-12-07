package src

import "unsafe"

const (
	DICT_OK  = 0
	DICT_ERR = 1
)

type dictTypeI interface {
	hashFunction(key unsafe.Pointer) uint64_t //redis用的是MurMurHash2
	keyDup(d *dict, key unsafe.Pointer) unsafe.Pointer
	valDup(d *dict, obj unsafe.Pointer) unsafe.Pointer
	keyCompare(d *dict, key1 unsafe.Pointer, keys unsafe.Pointer) int
	keyDestructor(d *dict, key unsafe.Pointer)
	valDestructor(d *dict, obj unsafe.Pointer)
	expandAllowed(moreMem size_t, usedRatio float64) int
	/* 允许dictEntry有由调用者定义的metadata，当dictEntry创建时，额外的metadata内存空间被初始化为0 */
	dictEntryMetadataBytes(d *dict) size_t
}
type dictType struct {
	hashFunction  func(key unsafe.Pointer) uint64_t //redis用的是MurMurHash2
	keyDup        func(d *dict, key unsafe.Pointer) unsafe.Pointer
	valDup        func(d *dict, obj unsafe.Pointer) unsafe.Pointer
	keyCompare    func(d *dict, key1 unsafe.Pointer, keys unsafe.Pointer) int
	keyDestructor func(d *dict, key unsafe.Pointer)
	valDestructor func(d *dict, obj unsafe.Pointer)
	expandAllowed func(moreMem size_t, usedRatio float64) int
	/* 允许dictEntry有由调用者定义的metadata，当dictEntry创建时，额外的metadata内存空间被初始化为0 */
	dictEntryMetadataBytes func(d *dict) size_t
}

type dict struct {
	typei dictType

	ht_table [2][]*dictEntry
	ht_used  [2]uint64 //[]*dictEntry已用个数

	rehashidx int64 /* rehashing not in progress if rehashidx == -1 */

	/* Keep small vars at end for optimal (minimal) struct padding */
	pauserehash int16_t /* 如果>0，则rehash暂停了，（<0代表程序写错了） */
	ht_size_exp [2]int8 /* exponent of size. (size = 1<<exp) */
}

type dictEntry struct {
	key unsafe.Pointer
	v   struct {
		val unsafe.Pointer
		u64 uint64_t
		s64 int64_t
		d   float64
	}
	next     *dictEntry        /* Next entry in the same hash bucket. */
	metadata *[]unsafe.Pointer /* An arbitrary number of bytes (starting at a
	 * pointer-aligned address) of size as returned
	 * by dictType's dictEntryMetadataBytes(). */
}

/***** C里的宏指令 ********/

func dictSize(d *dict) uint64 {
	return (*d).ht_used[0] + (*d).ht_used[1]
}

func dictIsRehashing(d *dict) bool {
	return d.rehashidx != -1
}
func dictHashKey(d *dict, key unsafe.Pointer) uint64_t {
	return d.typei.hashFunction(key)
}

func dictCompareKeys(d *dict, key1, key2 unsafe.Pointer) bool {
	//TODO 是否需要深度相等判断？
	if d.typei.keyCompare != nil {
		return d.typei.keyCompare(d, key1, key2) != 0
	}
	return key1 == key2
}

func dictFreeKey(d *dict, entry *dictEntry) {
	if d.typei.keyDestructor != nil {
		d.typei.keyDestructor(d, entry.key)
	}
}

func dictFreeVal(d *dict, entry *dictEntry) {
	if d.typei.valDestructor != nil {
		d.typei.valDestructor(d, entry.v.val)
	}
}

func dictGetVal(he *dictEntry) unsafe.Pointer {
	return he.v.val
}

func DICTHT_SIZE(exp int8) uint64 {
	if exp == -1 {
		return 0
	}

	return 1 << exp
}

func DICTHT_SIZE_MASK(exp int8) uint64 {
	if exp == -1 {
		return 0
	}

	return DICTHT_SIZE(exp) - 1
}

/***** C里的宏指令 ********/
