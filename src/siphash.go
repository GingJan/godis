package src

import (
	"errors"
)

const (
	UNALIGNED_LE_CPU = 1
)

func U8TO64_LE(p *uint8_t) uint64_t {
	//return ((uint64_t(((p)[0]))) | ((uint64_t)((p)[1]) << 8) |
	//((uint64_t)((p)[2]) << 16) | ((uint64_t)((p)[3]) << 24) |
	//((uint64_t)((p)[4]) << 32) | ((uint64_t)((p)[5]) << 40) |
	//((uint64_t)((p)[6]) << 48) | ((uint64_t)((p)[7]) << 56))
	return 0
}

func siphash(in *uint8_t, inlen size_t, k *[16]uint8_t) uint64_t {
	//TODO
	//if UNALIGNED_LE_CPU == 0 {
	//	var hash uint64_t
	//	var out *uint8_t  = (*uint8_t)(unsafe.Pointer(&hash))
	//} else {
	//	var v0 uint64_t = 0x736f6d6570736575
	//	var v1 uint64_t = 0x646f72616e646f6d
	//	var v2 uint64_t = 0x6c7967656e657261
	//	var v3 uint64_t = 0x7465646279746573
	//	var k0 uint64_t = U8TO64_LE(&(k[0]))
	//	var k1 uint64_t = U8TO64_LE(&(k[1]))
	//	var m uint64_t
	//	var end *uint8_t = size_t(in) + inlen - (inlen % unsafe.Sizeof(uint64_t))
	//	left := inlen & 7
	//	b := (uint64_t(inlen)) << 56
	//	v3 ^= k1
	//	v2 ^= k0
	//	v1 ^= k1
	//	v0 ^= k0
	//
	//	for ; in != end; in += 8 {
	//		m = U8TO64_LE(in)
	//		v3 ^= m
	//
	//		SIPROUND
	//
	//		v0 ^= m
	//	}
	//
	//	var bitval uint8
	//	if left > 0 {
	//		bitval, _ = getbitval(in, uint8(left-1))
	//	}
	//	switch left {
	//	case 7: b |= (uint64_t(bitval)) << 48;fallthrough/* fall-thru */
	//	case 6: b |= (uint64_t(bitval)) << 40;fallthrough /* fall-thru */
	//	case 5: b |= (uint64_t(bitval)) << 32;fallthrough /* fall-thru */
	//	case 4: b |= (uint64_t(bitval)) << 24;fallthrough /* fall-thru */
	//	case 3: b |= (uint64_t(bitval)) << 16;fallthrough /* fall-thru */
	//	case 2: b |= (uint64_t(bitval)) << 8;fallthrough /* fall-thru */
	//	case 1: b |= (uint64_t(bitval)); break
	//	case 0: break
	//	}
	//
	//	v3 ^= b
	//
	//	SIPROUND
	//
	//	v0 ^= b
	//	v2 ^= 0xff
	//
	//	SIPROUND
	//	SIPROUND
	//
	//	b = v0 ^ v1 ^ v2 ^ v3;
	//}
	//
	//if UNALIGNED_LE_CPU == 1 {
	//	U64TO8_LE(out, b)
	//	return hash
	//} else {
	//	return b
	//}
	return 0
}

func siphash_nocase(in *uint8_t, inlen size_t, k *[16]uint8_t) uint64_t {
	return 0
}

func getbitval(in interface{}, i uint8) (uint8, error) {
	if i < 0 {
		return 0, errors.New("i < 0 error")
	}

	i = i + 1
	var max uint8
	switch in.(type) {
	case uint8:
		max = 8
		inn := in.(uint8)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case int8:
		max = 8
		inn := in.(int8)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case uint16:
		max = 16
		inn := in.(uint16)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case int16:
		max = 16
		inn := in.(int16)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case uint32:
		max = 32
		inn := in.(uint32)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case int32:
		max = 32
		inn := in.(int32)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case uint64:
		max = 64
		inn := in.(uint64)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case int64:
		max = 64
		inn := in.(int64)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case int:
		max = 32 << (^uint(0) >> 63) //max=32 or 64
		inn := in.(int)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	case uint:
		max = 32 << (^uint(0) >> 63) //max=32 or 64
		inn := in.(uint)
		return uint8((inn << (max - i)) >> (max - 1)), nil
	default:
		return 0, errors.New("invalid in")
	}
}
