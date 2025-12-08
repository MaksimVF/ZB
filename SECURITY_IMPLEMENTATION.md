

# Security Implementation Guide

## 1. API Key Masking Implementation

### Current Issue:
In `auth-service/main.go`, the `getUserAPIKeys` function returns full API keys in responses.

### Fix:
```go
// Mask the API key for responses
func maskAPIKey(key string) string {
    if len(key) < 8 {
        return "****"
    }
    return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func getUserAPIKeys(userID string) []map[string]interface{} {
    var keys []APIKey
    if err := db.Where("user_id = ?", userID).Find(&keys).Error; err != nil {
        logger.Error().Err(err).Str("user_id", userID).Msg("Failed to get API keys")
        return []map[string]interface{}{}
    }

    var result []map[string]interface{}
    for _, k := range keys {
        result = append(result, map[string]interface{}{
            "id":      k.ID,
            "name":    k.Name,
            "key":     maskAPIKey(k.Key), // Mask the key
            "prefix":  k.Prefix,
            "created": k.Created,
        })
    }
    return result
}
```

## 2. Log Masking Implementation

### Current Issue:
Sensitive data is logged in plaintext.

### Fix:
```go
// Add to auth-service/main.go
func logSafeEmail(email string) string {
    if email == "" {
        return ""
    }
    parts := strings.Split(email, "@")
    if len(parts) != 2 {
        return "invalid-email"
    }
    username := parts[0]
    domain := parts[1]

    if len(username) <= 3 {
        return username + "@" + domain
    }
    return username[:2] + strings.Repeat("*", len(username)-2) + "@" + domain
}

// Usage example:
logger.Info().Str("email", logSafeEmail(req.Email)).Msg("User registration attempt")
```

## 3. Enhanced Rate Limiting

### Current Issue:
Simple in-memory rate limiting that doesn't scale.

### Fix:
```go
// Enhanced rate limiting middleware using Redis
func enhancedRateLimitMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        ip := getClientIP(r)
        key := fmt.Sprintf("rate_limit:%s", ip)

        // Check rate limit in Redis
        count, err := rdb.Get(context.Background(), key).Int()
        if err == nil && count >= 100 { // 100 requests per minute
            logger.Warn().Str("ip", ip).Msg("Rate limit exceeded")
            w.Header().Set("Retry-After", "60")
            http.Error(w, RateLimitExceededError, 429)
            httpDuration.WithLabelValues(r.Method, r.URL.Path, "429").Observe(time.Since(start).Seconds())
            return
        }

        // Increment counter with expiration
        pipe := rdb.TxPipeline()
        pipe.Incr(context.Background(), key)
        pipe.Expire(context.Background(), key, 1*time.Minute)
        _, err = pipe.Exec(context.Background())
        if err != nil {
            logger.Error().Err(err).Msg("Failed to increment rate limit counter")
            http.Error(w, InternalServerError, 500)
            httpDuration.WithLabelValues(r.Method, r.URL.Path, "500").Observe(time.Since(start).Seconds())
            return
        }

        next.ServeHTTP(w, r)
    }
}

func getClientIP(r *http.Request) string {
    ip := r.Header.Get("X-Forwarded-For")
    if ip == "" {
        ip = r.RemoteAddr
    }
    // Handle multiple IPs in X-Forwarded-For
    if strings.Contains(ip, ",") {
        ips := strings.Split(ip, ",")
        return strings.TrimSpace(ips[0])
    }
    return strings.TrimSpace(ip)
}
```

## 4. Security Headers Middleware

### Current Issue:
Missing security headers in HTTP responses.

### Fix:
```go
// Security headers middleware
func securityHeadersMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Content Security Policy
        w.Header().Set("Content-Security-Policy",
            "default-src 'self'; script-src 'self' 'unsafe-inline' 'unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;")

        // XSS Protection
        w.Header().Set("X-XSS-Protection", "1; mode=block")

        // Clickjacking protection
        w.Header().Set("X-Frame-Options", "DENY")

        // MIME type sniffing protection
        w.Header().Set("X-Content-Type-Options", "nosniff")

        // Referrer Policy
        w.Header().Set("Referrer-Policy", "no-referrer")

        // Permissions Policy
        w.Header().Set("Permissions-Policy",
            "geolocation=(), microphone=(), camera=(), payment=()")

        // Strict Transport Security
        w.Header().Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")

        next.ServeHTTP(w, r)
    })
}

// Usage in main:
r.Use(securityHeadersMiddleware)
```

## 5. Input Validation Enhancement

### Current Issue:
Insufficient input validation.

### Fix:
```go
// Enhanced input validation
func isValidEmail(email string) bool {
    // RFC 5322 compliant regex
    emailRegex := `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`
    return regexMatch(emailRegex, email)
}

func isStrongPassword(password string) bool {
    if len(password) < 12 {
        return false
    }

    hasUpper := false
    hasLower := false
    hasNumber := false
    hasSpecial := false

    for _, char := range password {
        switch {
        case unicode.IsUpper(char):
            hasUpper = true
        case unicode.IsLower(char):
            hasLower = true
        case unicode.IsNumber(char):
            hasNumber = true
        case unicode.IsPunct(char) || unicode.IsSymbol(char):
            hasSpecial = true
        }
    }

    return hasUpper && hasLower && hasNumber && hasSpecial
}

func sanitizeInput(input string) string {
    // Remove potentially dangerous characters
    return strings.TrimSpace(strings.Map(func(r rune) rune {
        if r == '\n' || r == '\r' || r == '\t' {
            return -1
        }
        return r
    }, input))
}
```

## 6. CORS Configuration Fix

### Current Issue:
Overly permissive CORS settings.

### Fix:
```go
// Proper CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Allow only specific domains
        allowedOrigins := []string{
            "https://yourdomain.com",
            "https://api.yourdomain.com",
        }

        origin := r.Header.Get("Origin")
        isAllowed := false
        for _, allowed := range allowedOrigins {
            if origin == allowed {
                isAllowed = true
                break
            }
        }

        if isAllowed {
            w.Header().Set("Access-Control-Allow-Origin", origin)
            w.Header().Set("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
            w.Header().Set("Access-Control-Allow-Headers", "Content-Type,Authorization,X-Admin-Key")
            w.Header().Set("Access-Control-Allow-Credentials", "true")
            w.Header().Set("Access-Control-Max-Age", "86400")
        }

        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusNoContent)
            return
        }

        next.ServeHTTP(w, r)
    })
}
```

## 7. JWT Security Enhancements

### Current Issue:
Basic JWT implementation without proper security measures.

### Fix:
```go
// Enhanced JWT creation and validation
func createJWT(user User) (string, error) {
    token := jwt.NewWithClaims(jwt.SigningMethodHS512, jwt.MapClaims{
        "user_id": user.ID,
        "email":   logSafeEmail(user.Email), // Mask email in token
        "role":    user.Role,
        "exp":     time.Now().Add(30 * 24 * time.Hour).Unix(),
        "iat":     time.Now().Unix(),
        "jti":     uuid.New().String(), // Unique token identifier
    })

    return token.SignedString(secret)
}

func validateJWT(tokenStr string) (*jwt.Token, error) {
    return jwt.Parse(tokenStr, func(token *jwt.Token) (interface{}, error) {
        // Validate signing method
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("invalid signing method")
        }

        // Validate token structure
        if _, ok := token.Claims.(jwt.MapClaims); !ok {
            return nil, fmt.Errorf("invalid token claims")
        }

        return secret, nil
    })
}
```

## 8. Error Handling Security

### Current Issue:
Error responses may leak sensitive information.

### Fix:
```go
// Secure error handling
func handleError(w http.ResponseWriter, err error, statusCode int, message string) {
    logger.Error().Err(err).Msg(message)

    // Generic error response to avoid information leakage
    response := struct {
        Error   string `json:"error"`
        Code    int    `json:"code"`
        RequestID string `json:"request_id"`
    }{
        Error:   http.StatusText(statusCode),
        Code:    statusCode,
        RequestID: uuid.New().String(),
    }

    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(statusCode)
    json.NewEncoder(w).Encode(response)
}

// Usage:
// if err != nil {
//     handleError(w, err, http.StatusInternalServerError, "Database operation failed")
//     return
// }
```

## 9. Request Size Limiting

### Current Issue:
No request size limits, potential for DoS attacks.

### Fix:
```go
// Request size limiting middleware
func requestSizeLimitMiddleware(maxSize int64) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Check content length
            if r.ContentLength > maxSize {
                http.Error(w, "Request entity too large", http.StatusRequestEntityTooLarge)
                return
            }

            // For requests without content-length, limit body reading
            r.Body = http.MaxBytesReader(w, r.Body, maxSize)

            next.ServeHTTP(w, r)
        })
    }
}

// Usage:
// r.Use(requestSizeLimitMiddleware(10 << 20)) // 10MB limit
```

## 10. Security Configuration Checklist

### Required Environment Variables:
```
# Auth service
JWT_SECRET=your-very-secure-secret-key-here
JWT_EXPIRATION=720h
JWT_ISSUER=your-company-name

# Rate limiting
RATE_LIMIT_REDIS_URL=redis://redis:6379
MAX_REQUESTS_PER_MINUTE=100

# CORS
ALLOWED_ORIGINS=https://yourdomain.com,https://api.yourdomain.com

# Security headers
CSP_POLICY="default-src 'self'; script-src 'self' 'unsafe-inline'; style-src 'self' 'unsafe-inline'; img-src 'self' data:;"
```

### Implementation Steps:

1. **Update auth-service**:
   - Mask API keys in responses
   - Implement proper log masking
   - Add enhanced input validation
   - Implement secure JWT handling

2. **Update gateway**:
   - Add security headers middleware
   - Implement CORS restrictions
   - Add request size limiting

3. **Update rate-limiter**:
   - Implement distributed rate limiting
   - Add IP-based tracking
   - Implement proper rate limit headers

4. **Update all services**:
   - Add security headers middleware
   - Implement proper error handling
   - Add request validation

5. **Update deployment**:
   - Configure proper environment variables
   - Implement network security policies
   - Configure proper TLS certificates

