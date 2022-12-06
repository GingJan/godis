package src

import "unsafe"

type dictType interface {
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

type dict struct {
	typei dictType

	ht_table [2][]*dictEntry
	ht_used [2]uint64

	rehashidx int64 /* rehashing not in progress if rehashidx == -1 */

	/* Keep small vars at end for optimal (minimal) struct padding */
	pauserehash int16_t/* If >0 rehashing is paused (<0 indicates coding error) */
	ht_size_exp [2]int8 /* exponent of size. (size = 1<<exp) */
}

type dictEntry struct {
	key unsafe.Pointer
	v struct{
		val unsafe.Pointer
		u64 uint64_t
		s64 int64_t
		d float64
	}
	next *dictEntry     /* Next entry in the same hash bucket. */
	metadata *[]unsafe.Pointer           /* An arbitrary number of bytes (starting at a
	 * pointer-aligned address) of size as returned
	 * by dictType's dictEntryMetadataBytes(). */
}

func dictSize(d *dict) uint64 {
	return (*d).ht_used[0] + (*d).ht_used[1]
}

func dictIsRehashing(d *dict) bool {
	return d.rehashidx != -1
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