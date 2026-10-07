package shrt

import "crypto/rand"

// codeAlphabet holds the characters of a generated code (ADR 0004).
const codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

const codeLength = 6

// newCode draws a random code of codeLength characters from codeAlphabet.
// Random bytes of 248 or more are dropped, so that every character is
// equally likely: 248 is the largest multiple of 62 a byte holds.
func newCode() string {
	const limit = 256 - 256%len(codeAlphabet)
	code := make([]byte, 0, codeLength)
	buf := make([]byte, 2*codeLength)
	for len(code) < codeLength {
		rand.Read(buf)
		for _, b := range buf {
			if int(b) < limit && len(code) < codeLength {
				code = append(code, codeAlphabet[int(b)%len(codeAlphabet)])
			}
		}
	}
	return string(code)
}
