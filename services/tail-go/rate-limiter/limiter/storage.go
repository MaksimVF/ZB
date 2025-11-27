// limiter/storage.go
package limiter

import (
"context"
"sync"
"time"
)

var mu sync.RWMutex
var rules = map[string]Rule{}

type Rule struct {
MaxTokens int64
Burst     int64
}

func CheckTokens(clientID, path string, tokens int64) bool {
mu.RLock()
rule := rules[path+":tokens"]
mu.RUnlock()
return tokens <= rule.Burst
}
