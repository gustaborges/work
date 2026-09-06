// Package id generates opaque, lexicographically-sortable identifiers for
// Works. The format is ULID: a 48-bit millisecond timestamp followed by 80
// bits of randomness, Crockford base32-encoded to 26 characters. Sorting by
// string orders by creation time.
package id

import (
	"crypto/rand"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// New returns a new ULID for the given time.
func New(t time.Time) string {
	ms := uint64(t.UTC().UnixMilli())

	var b [16]byte
	// 48-bit timestamp, big-endian, in the first 6 bytes.
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	_, _ = rand.Read(b[6:])

	// Encode 128 bits as 26 base32 chars (the first char carries only 2 bits).
	var out [26]byte
	out[0] = crockford[(b[0]&0xE0)>>5]
	out[1] = crockford[b[0]&0x1F]
	out[2] = crockford[(b[1]&0xF8)>>3]
	out[3] = crockford[((b[1]&0x07)<<2)|((b[2]&0xC0)>>6)]
	out[4] = crockford[(b[2]&0x3E)>>1]
	out[5] = crockford[((b[2]&0x01)<<4)|((b[3]&0xF0)>>4)]
	out[6] = crockford[((b[3]&0x0F)<<1)|((b[4]&0x80)>>7)]
	out[7] = crockford[(b[4]&0x7C)>>2]
	out[8] = crockford[((b[4]&0x03)<<3)|((b[5]&0xE0)>>5)]
	out[9] = crockford[b[5]&0x1F]
	out[10] = crockford[(b[6]&0xF8)>>3]
	out[11] = crockford[((b[6]&0x07)<<2)|((b[7]&0xC0)>>6)]
	out[12] = crockford[(b[7]&0x3E)>>1]
	out[13] = crockford[((b[7]&0x01)<<4)|((b[8]&0xF0)>>4)]
	out[14] = crockford[((b[8]&0x0F)<<1)|((b[9]&0x80)>>7)]
	out[15] = crockford[(b[9]&0x7C)>>2]
	out[16] = crockford[((b[9]&0x03)<<3)|((b[10]&0xE0)>>5)]
	out[17] = crockford[b[10]&0x1F]
	out[18] = crockford[(b[11]&0xF8)>>3]
	out[19] = crockford[((b[11]&0x07)<<2)|((b[12]&0xC0)>>6)]
	out[20] = crockford[(b[12]&0x3E)>>1]
	out[21] = crockford[((b[12]&0x01)<<4)|((b[13]&0xF0)>>4)]
	out[22] = crockford[((b[13]&0x0F)<<1)|((b[14]&0x80)>>7)]
	out[23] = crockford[(b[14]&0x7C)>>2]
	out[24] = crockford[((b[14]&0x03)<<3)|((b[15]&0xE0)>>5)]
	out[25] = crockford[b[15]&0x1F]
	return string(out[:])
}
