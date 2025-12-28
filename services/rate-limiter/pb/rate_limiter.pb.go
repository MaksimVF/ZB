package pb

import (
	"context"
)

// Generated type definitions for rate_limiter.proto
type CheckRequest struct {
	Authorization string `protobuf:"bytes,1,opt,name=authorization" json:"authorization,omitempty"`
	Path          string `protobuf:"bytes,2,opt,name=path" json:"path,omitempty"`
}

type CheckResponse struct {
	Allowed        bool   `protobuf:"varint,1,opt,name=allowed" json:"allowed,omitempty"`
	RetryAfterSecs uint32 `protobuf:"varint,2,opt,name=retry_after_secs" json:"retry_after_secs,omitempty"`
}

type SetLimitRequest struct {
	Path     string `protobuf:"bytes,1,opt,name=path" json:"path,omitempty"`
	AuthType string `protobuf:"bytes,2,opt,name=auth_type" json:"auth_type,omitempty"`
	Limit    int32  `protobuf:"varint,3,opt,name=limit" json:"limit,omitempty"`
}

type SetLimitResponse struct {
	Success bool   `protobuf:"varint,1,opt,name=success" json:"success,omitempty"`
	Message string `protobuf:"bytes,2,opt,name=message" json:"message,omitempty"`
}

type GetLimitsRequest struct{}

type GetLimitsResponse struct {
	Limits map[string]*LimitConfig `protobuf:"bytes,1,rep,name=limits" json:"limits,omitempty"`
}

type LimitConfig struct {
	Path           string `protobuf:"bytes,1,opt,name=path" json:"path,omitempty"`
	JwtLimit       int32  `protobuf:"varint,2,opt,name=jwt_limit" json:"jwt_limit,omitempty"`
	ApiKeyLimit    int32  `protobuf:"varint,3,opt,name=api_key_limit" json:"api_key_limit,omitempty"`
	AnonymousLimit int32  `protobuf:"varint,4,opt,name=anonymous_limit" json:"anonymous_limit,omitempty"`
}

type RateLimiterServer interface {
	Check(context.Context, *CheckRequest) (*CheckResponse, error)
	SetLimit(context.Context, *SetLimitRequest) (*SetLimitResponse, error)
	GetLimits(context.Context, *GetLimitsRequest) (*GetLimitsResponse, error)
}

type UnimplementedRateLimiterServer struct{}

func (*UnimplementedRateLimiterServer) Check(context.Context, *CheckRequest) (*CheckResponse, error) {
	return nil, nil
}

func (*UnimplementedRateLimiterServer) SetLimit(context.Context, *SetLimitRequest) (*SetLimitResponse, error) {
	return nil, nil
}

func (*UnimplementedRateLimiterServer) GetLimits(context.Context, *GetLimitsRequest) (*GetLimitsResponse, error) {
	return nil, nil
}

func RegisterRateLimiterServer(s interface{}, server RateLimiterServer) {
	// Registration function placeholder
}
