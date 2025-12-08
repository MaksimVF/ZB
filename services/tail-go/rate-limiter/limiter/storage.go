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
rule, exists := rules[path+":tokens"]
mu.RUnlock()
if !exists {
return tokens <= 1000 // Default burst of 1000 tokens
}
return tokens <= rule.Burst
}
