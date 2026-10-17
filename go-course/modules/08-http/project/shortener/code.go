package shortener

import "crypto/rand"

// codeAlphabet is URL-safe: no characters need escaping in a path segment.
const codeAlphabet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// codeLen gives 62^7 ≈ 3.5e12 possible codes.
const codeLen = 7

// RandomCode returns a random 7-character code from [a-zA-Z0-9].
func RandomCode() (string, error) {
	var b [codeLen]byte
	// crypto/rand.Read never returns an error as of Go 1.24, but the
	// signature still has one; keep the check for older toolchains.
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	for i := range b {
		// 256 is not a multiple of 62, so this has a slight bias towards the
		// first 8 letters. Acceptable for short links, not for secrets.
		b[i] = codeAlphabet[int(b[i])%len(codeAlphabet)]
	}
	return string(b[:]), nil
}
