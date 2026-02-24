package utils

import (
	"fmt"
	"sync/atomic"
	"time"
)

var counter uint64

func GenerateID() string {
	newCounter := atomic.AddUint64(&counter, 1)
	return fmt.Sprintf("msg_%d_%d", time.Now().UnixNano(), newCounter)
}
