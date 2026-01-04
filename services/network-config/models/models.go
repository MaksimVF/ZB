package models

import (
	"encoding/json"
	"time"
)

// NetworkConfig represents the network configuration structure
type NetworkConfig struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name" validate:"required"`
	Description string    `json:"description" db:"description"`
	Mode        string    `json:"mode" db:"mode" validate:"required,oneof=direct wireguard tailscale zerotier"`
	Version     int       `json:"version" db:"version"`
	Status      string    `json:"status" db:"status" validate:"required,oneof=active inactive deprecated"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`

	// Network settings
	HeadEndpoints []EndpointConfig    `json:"head_endpoints" db:"-"`
	LoadBalancing LoadBalancingConfig `json:"load_balancing" db:"-"`
	RetryPolicy   RetryPolicy         `json:"retry_policy" db:"-"`
	RateLimits    RateLimitsConfig    `json:"rate_limits" db:"-"`

	// WireGuard settings
	WGPeerPublic string `json:"wg_peer_public,omitempty" db:"wg_peer_public"`
	WGAllowedIPs string `json:"wg_allowed_ips,omitempty" db:"wg_allowed_ips"`

	// Tailscale settings
	TailscaleAuthKey         string `json:"tailscale_auth_key,omitempty" db:"tailscale_auth_key"`
	TailscaleHostname        string `json:"tailscale_hostname,omitempty" db:"tailscale_hostname"`
	TailscaleAdvertiseRoutes string `json:"tailscale_advertise_routes,omitempty" db:"tailscale_advertise_routes"`

	// Security
	SecurityToken string `json:"security_token" db:"security_token" validate:"required"`

	// Metadata
	Tags        []string          `json:"tags" db:"-"`
	Labels      map[string]string `json:"labels" db:"-"`
	Annotations map[string]string `json:"annotations" db:"-"`
}

// EndpointConfig represents endpoint configuration
type EndpointConfig struct {
	Name        string            `json:"name" validate:"required"`
	URL         string            `json:"url" validate:"required,url"`
	Protocol    string            `json:"protocol" validate:"required,oneof=grpc http https"`
	Host        string            `json:"host" validate:"required"`
	Port        int               `json:"port" validate:"required,min=1,max=65535"`
	Weight      int               `json:"weight" validate:"min=0,max=100"`
	HealthCheck HealthCheckConfig `json:"health_check"`
	IsActive    bool              `json:"is_active" default:"true"`
}

// HealthCheckConfig represents health check configuration
type HealthCheckConfig struct {
	Enabled  bool          `json:"enabled" default:"true"`
	Interval time.Duration `json:"interval" default:"30s"`
	Timeout  time.Duration `json:"timeout" default:"5s"`
	Path     string        `json:"path" default:"/health"`
	Method   string        `json:"method" default:"GET"`
	Retries  int           `json:"retries" default:"3"`
}

// LoadBalancingConfig represents load balancing configuration
type LoadBalancingConfig struct {
	Strategy        string `json:"strategy" validate:"required,oneof=round_robin least_connections weighted_random ip_hash"`
	StickySessions  bool   `json:"sticky_sessions" default:"false"`
	HealthCheck     bool   `json:"health_check" default:"true"`
	CheckInterval   int    `json:"check_interval" default:"30"`
	FailoverEnabled bool   `json:"failover_enabled" default:"true"`
}

// RetryPolicy represents retry policy configuration
type RetryPolicy struct {
	MaxRetries         int      `json:"max_retries" validate:"min=0,max=10"`
	BackoffFactor      int      `json:"backoff_factor" validate:"min=1,max=10"`
	MaxBackoff         int      `json:"max_backoff" validate:"min=1,max=60"`
	RetryOnStatus      []int    `json:"retry_on_status" default:"[500,502,503,504]"`
	RetryOnErrors      []string `json:"retry_on_errors"`
	ExponentialBackoff bool     `json:"exponential_backoff" default:"true"`
}

// RateLimitsConfig represents rate limiting configuration
type RateLimitsConfig struct {
	Enabled           bool     `json:"enabled" default:"true"`
	RequestsPerSecond int      `json:"requests_per_second" validate:"min=1"`
	RequestsPerMinute int      `json:"requests_per_minute" validate:"min=1"`
	RequestsPerHour   int      `json:"requests_per_hour" validate:"min=1"`
	Burst             int      `json:"burst" validate:"min=1"`
	Whitelist         []string `json:"whitelist"`
	Blacklist         []string `json:"blacklist"`
}

// ConfigHistory represents configuration history
type ConfigHistory struct {
	ID        string    `json:"id" db:"id"`
	ConfigID  string    `json:"config_id" db:"config_id"`
	Version   int       `json:"version" db:"version"`
	Changes   string    `json:"changes" db:"changes"`
	CreatedBy string    `json:"created_by" db:"created_by"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`

	// Full config snapshot
	ConfigData json.RawMessage `json:"config_data" db:"config_data"`
}

// NetworkStatus represents current network status
type NetworkStatus struct {
	ID                string    `json:"id"`
	ConfigID          string    `json:"config_id"`
	Status            string    `json:"status" validate:"required,oneof=healthy degraded unhealthy unknown"`
	Message           string    `json:"message"`
	LastCheck         time.Time `json:"last_check"`
	Uptime            float64   `json:"uptime"`
	ResponseTime      float64   `json:"response_time"`
	ActiveConnections int       `json:"active_connections"`

	// Endpoint status
	Endpoints []EndpointStatus `json:"endpoints"`
}

// EndpointStatus represents endpoint status
type EndpointStatus struct {
	Name         string    `json:"name"`
	URL          string    `json:"url"`
	Status       string    `json:"status" validate:"required,oneof=healthy unhealthy unknown"`
	LastCheck    time.Time `json:"last_check"`
	ResponseTime float64   `json:"response_time"`
	Error        string    `json:"error"`
}

// TailscaleStatus represents Tailscale connection status
type TailscaleStatus struct {
	Connected    bool      `json:"connected"`
	IPAddress    string    `json:"ip_address"`
	Hostname     string    `json:"hostname"`
	Version      string    `json:"version"`
	PeerCount    int       `json:"peer_count"`
	LastActivity time.Time `json:"last_activity"`
	Routes       []string  `json:"routes"`
	State        string    `json:"state"`
}

// ErrorResponse represents API error response
type ErrorResponse struct {
	Code    int                    `json:"code"`
	Message string                 `json:"message"`
	Error   string                 `json:"error,omitempty"`
	Details map[string]interface{} `json:"details,omitempty"`
	TraceID string                 `json:"trace_id,omitempty"`
	Time    time.Time              `json:"time"`
}

// SuccessResponse represents API success response
type SuccessResponse struct {
	Data    interface{} `json:"data"`
	Message string      `json:"message,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
	Time    time.Time   `json:"time"`
}

// ValidationError represents validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
	Code    string `json:"code"`
}

// APIResponse represents unified API response
type APIResponse struct {
	Success    bool                   `json:"success"`
	Data       interface{}            `json:"data,omitempty"`
	Error      *ErrorResponse         `json:"error,omitempty"`
	Pagination *Pagination            `json:"pagination,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

// Pagination represents pagination metadata
type Pagination struct {
	Page       int  `json:"page"`
	PerPage    int  `json:"per_page"`
	Total      int  `json:"total"`
	TotalPages int  `json:"total_pages"`
	HasNext    bool `json:"has_next"`
	HasPrev    bool `json:"has_prev"`
}

// ConfigUpdateRequest represents configuration update request
type ConfigUpdateRequest struct {
	Config *NetworkConfig `json:"config" validate:"required"`
	Force  bool           `json:"force" default:"false"`
}

// HealthCheck represents service health check
type HealthCheck struct {
	Status    string             `json:"status"`
	Timestamp time.Time          `json:"timestamp"`
	Checks    map[string]string  `json:"checks"`
	Metrics   map[string]float64 `json:"metrics"`
}

// ServiceInfo represents service information
type ServiceInfo struct {
	Name        string    `json:"name"`
	Version     string    `json:"version"`
	Build       string    `json:"build"`
	Environment string    `json:"environment"`
	Uptime      float64   `json:"uptime"`
	StartedAt   time.Time `json:"started_at"`
	GitCommit   string    `json:"git_commit"`
}
