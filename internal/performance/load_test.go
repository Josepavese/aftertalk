package performance

import (
	"math/rand"
	"time"
)

var loadTestDBPath string

func init() {
	rand.Seed(time.Now().UnixNano())
	loadTestDBPath = "/tmp/load_aftertalk.db"
}

