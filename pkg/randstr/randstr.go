package randstr

import (
	"crypto/rand"
	"encoding/base64"
	"io"
	"math"
	"strings"
)

func RandomString(minbits int) (string) {
	randbytes := make([]byte, int(math.Ceil(float64(minbits)/8)))
	_, _ = io.ReadFull(rand.Reader, randbytes)
	str := base64.StdEncoding.EncodeToString(randbytes)
	return strings.TrimRight(str, "=")
}
