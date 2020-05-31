package randstr

import (
	"crypto/rand"
	"encoding/base64"
	"io"
)

// RandomString returns a random string containing at least minbits of entropy.
// The string is base64 encoded and its length will be a multiple of 3 to avoid == characters.
func RandomString(length int) string {
	randbytes := make([]byte, length) // this is more than we need
	_, _ = io.ReadFull(rand.Reader, randbytes)
	str := base64.StdEncoding.EncodeToString(randbytes)
	return str[:length]
}
