package randstr

import (
	"crypto/rand"
	"fmt"
	"math/big"
)

// RandomString returns a random string of a given length.
func RandomString(length int) string {
	charset := "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	randbytes := make([]byte, 0, length)
	for i := 0; i < length; i++ {
		idx, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		randbytes = append(randbytes, charset[idx.Int64()])
	}

	return string(randbytes)
}

func RandomStringWithPrefixAndSuffix(prefix string, length int, suffix string) string {
	return fmt.Sprintf("%s%s%s", prefix, RandomString(length), suffix)
}
