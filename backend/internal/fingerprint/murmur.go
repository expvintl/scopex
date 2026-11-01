package fingerprint

import (
	"encoding/base64"
	"strings"
)

// MurmurHash3 x86 32-bit，种子 0 —— favicon 哈希按 32 位有符号整数输出，便于跨库检索。
func murmur3_32(data []byte, seed uint32) uint32 {
	const (
		c1 = 0xcc9e2d51
		c2 = 0x1b873593
	)
	h1 := seed
	nblocks := len(data) / 4
	for i := 0; i < nblocks; i++ {
		k1 := uint32(data[i*4]) | uint32(data[i*4+1])<<8 | uint32(data[i*4+2])<<16 | uint32(data[i*4+3])<<24
		k1 *= c1
		k1 = k1<<15 | k1>>17
		k1 *= c2
		h1 ^= k1
		h1 = h1<<13 | h1>>19
		h1 = h1*5 + 0xe6546b64
	}
	var k1 uint32
	tail := data[nblocks*4:]
	switch len(tail) {
	case 3:
		k1 ^= uint32(tail[2]) << 16
		fallthrough
	case 2:
		k1 ^= uint32(tail[1]) << 8
		fallthrough
	case 1:
		k1 ^= uint32(tail[0])
		k1 *= c1
		k1 = k1<<15 | k1>>17
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint32(len(data))
	h1 ^= h1 >> 16
	h1 *= 0x85ebca6b
	h1 ^= h1 >> 13
	h1 *= 0xc2b2ae35
	h1 ^= h1 >> 16
	return h1
}

// FaviconHash 复刻 favicon 指纹算法：base64 后按 76 字符换行，再取 murmur3。
func FaviconHash(data []byte) int32 {
	encoded := base64.StdEncoding.EncodeToString(data)
	var sb strings.Builder
	sb.Grow(len(encoded) + len(encoded)/76 + 2)
	for i := 0; i < len(encoded); i += 76 {
		end := i + 76
		if end > len(encoded) {
			end = len(encoded)
		}
		sb.WriteString(encoded[i:end])
		sb.WriteByte('\n')
	}
	return int32(murmur3_32([]byte(sb.String()), 0))
}
