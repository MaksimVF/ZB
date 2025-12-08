


package auth

import (
    "context"
    "errors"
    "strings"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "google.golang.org/grpc"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/metadata"
    "google.golang.org/grpc/status"
)

// TokenClaims represents the JWT token claims
type TokenClaims struct {
    UserID    string   `json:"user_id"`
    Roles     []string `json:"roles"`
    ExpiresAt int64    `json:"exp"`
    jwt.RegisteredClaims
}

// AuthConfig holds authentication configuration
type AuthConfig struct {
    JWTSecret       string
    TokenExpiration time.Duration
}

// Authenticator handles token-based authentication
type Authenticator struct {
    config AuthConfig
}

// NewAuthenticator creates a new authenticator
func NewAuthenticator(config AuthConfig) *Authenticator {
    return &Authenticator{config: config}
}

// GenerateToken creates a new JWT token
func (a *Authenticator) GenerateToken(userID string, roles []string) (string, error) {
    expiration := time.Now().Add(a.config.TokenExpiration).Unix()

    claims := TokenClaims{
        UserID:    userID,
        Roles:     roles,
        ExpiresAt: expiration,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Unix(expiration, 0)),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(a.config.JWTSecret))
}

// ValidateToken validates a JWT token
func (a *Authenticator) ValidateToken(tokenString string) (*TokenClaims, error) {
    claims := &TokenClaims{}

    token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
        if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, errors.New("unexpected signing method")
        }
        return []byte(a.config.JWTSecret), nil
    })

    if err != nil {
        return nil, err
    }

    if !token.Valid {
        return nil, errors.New("invalid token")
    }

    return claims, nil
}

// UnaryServerInterceptor provides gRPC unary server authentication
func (a *Authenticator) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(
        ctx context.Context,
        req interface{},
        info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler,
    ) (interface{}, error) {
        // Extract token from metadata
        md, ok := metadata.FromIncomingContext(ctx)
        if !ok {
            return nil, status.Errorf(codes.Unauthenticated, "missing metadata")
        }

        authHeader, ok := md["authorization"]
        if !ok || len(authHeader) == 0 {
            return nil, status.Errorf(codes.Unauthenticated, "missing authorization header")
        }

        token := strings.TrimPrefix(authHeader[0], "Bearer ")
        if token == authHeader[0] {
            return nil, status.Errorf(codes.Unauthenticated, "invalid authorization header format")
        }

        // Validate token
        claims, err := a.ValidateToken(token)
        if err != nil {
            return nil, status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
        }

        // Add claims to context
        newCtx := context.WithValue(ctx, "claims", claims)

        // Call the handler
        return handler(newCtx, req)
    }
}

// StreamServerInterceptor provides gRPC stream server authentication
func (a *Authenticator) StreamServerInterceptor() grpc.StreamServerInterceptor {
    return func(
        srv interface{},
        ss grpc.ServerStream,
        info *grpc.StreamServerInfo,
        handler grpc.StreamHandler,
    ) error {
        // Extract token from metadata
        md, ok := metadata.FromIncomingContext(ss.Context())
        if !ok {
            return status.Errorf(codes.Unauthenticated, "missing metadata")
        }

        authHeader, ok := md["authorization"]
        if !ok || len(authHeader) == 0 {
            return status.Errorf(codes.Unauthenticated, "missing authorization header")
        }

        token := strings.TrimPrefix(authHeader[0], "Bearer ")
        if token == authHeader[0] {
            return status.Errorf(codes.Unauthenticated, "invalid authorization header format")
        }

        // Validate token
        claims, err := a.ValidateToken(token)
        if err != nil {
            return status.Errorf(codes.Unauthenticated, "invalid token: %v", err)
        }

        // Add claims to context
        newCtx := context.WithValue(ss.Context(), "claims", claims)
        wrappedStream := &wrappedServerStream{ServerStream: ss, ctx: newCtx}

        // Call the handler
        return handler(srv, wrappedStream)
    }
}

// wrappedServerStream wraps the original stream to provide the new context
type wrappedServerStream struct {
    grpc.ServerStream
    ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
    return w.ctx
}

// GetClaimsFromContext extracts token claims from context
func GetClaimsFromContext(ctx context.Context) (*TokenClaims, bool) {
    claims, ok := ctx.Value("claims").(*TokenClaims)
    return claims, ok
}


