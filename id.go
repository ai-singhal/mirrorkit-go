package mirrorkit

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"
)

// genID returns a random hex id for a trace. It falls back to a timestamp-based
// id if the system RNG is unavailable (which essentially never happens).
func genID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "t" + strconv.FormatInt(time.Now().UnixNano(), 36)
	}
	return hex.EncodeToString(b)
}
