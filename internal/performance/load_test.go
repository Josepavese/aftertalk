package performance

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/flowup/aftertalk/internal/core/session"
	"github.com/flowup/aftertalk/internal/storage/cache"
	"github.com/flowup/aftertalk/internal/storage/sqlite"
	"github.com/flowup/aftertalk/pkg/jwt"
)

var loadTestDBPath string

func init() {
	rand.Seed(time.Now().UnixNano())
	loadTestDBPath = "/tmp/load_aftertalk.db"
}

