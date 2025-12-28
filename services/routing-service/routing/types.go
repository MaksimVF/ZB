package routing

// HeadService represents a head service
type HeadService struct {
	HeadID        string
	Endpoint      string
	Status        string
	CurrentLoad   int32
	Region        string
	ModelType     string
	Version       string
	Metadata      map[string]string
	LastHeartbeat int64
}

// RoutingPolicy defines the routing policy
type RoutingPolicy struct {
	DefaultStrategy     string
	EnableGeoRouting    bool
	EnableLoadBalancing bool
	EnableModelSpecific bool
	EnablePredictive    bool
	EnableAdaptive      bool
	StrategyConfig      map[string]string
	PredictionWindow    int
	LoadGrowthFactor    float64
	CapacityThreshold   float64
}

// HeadRegistry interface for head service registry
type HeadRegistry interface {
	Register(head HeadService) error
	UpdateStatus(headID, status string, load int32, timestamp int64) error
	GetAll() []HeadService
	GetByModelType(modelType string) []HeadService
	GetByRegion(region string) []HeadService
	GetActive() []HeadService
}

// RoutingRequest represents a routing decision request
type RoutingRequest struct {
	ClientID         string
	ModelType        string
	RegionPreference string
	RoutingStrategy  string
	Metadata         map[string]string
}

// RoutingResponse represents a routing decision response
type RoutingResponse struct {
	HeadID       string
	Endpoint     string
	StrategyUsed string
	Reason       string
	Metadata     map[string]string
}

// PolicyManager interface for routing policy management
type PolicyManager interface {
	Get() *RoutingPolicy
	Update(policy *RoutingPolicy) error
}
