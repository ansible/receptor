package randstr

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"math"
	"strings"
)

// RandomString returns a random string containing at least minbits of entropy.
// The string is base64 encoded and its length will be a multiple of 3 to avoid == characters.
func RandomString(minbits int) (string) {
	randbytes := make([]byte, int(math.Ceil(float64(minbits)/8)))
	_, _ = io.ReadFull(rand.Reader, randbytes)
	str := base64.StdEncoding.EncodeToString(randbytes)
	return strings.TrimRight(str, "=")
}
