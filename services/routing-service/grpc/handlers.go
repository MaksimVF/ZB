package grpc

import (
	"context"
	"time"

	pb "github.com/MaksimVF/ZB/services/routing-service/proto"
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// GRPCHandlers implements gRPC service handlers
type GRPCHandlers struct {
	routingEngine *routing.RoutingEngine
	registry      routing.HeadRegistry
	policyMgr     routing.PolicyManager
}

// NewGRPCHandlers creates new gRPC handlers
func NewGRPCHandlers(engine *routing.RoutingEngine, registry routing.HeadRegistry, policyMgr routing.PolicyManager) *GRPCHandlers {
	return &GRPCHandlers{
		routingEngine: engine,
		registry:      registry,
		policyMgr:     policyMgr,
	}
}

// RegisterHead implements the RegisterHead RPC method
func (h *GRPCHandlers) RegisterHead(ctx context.Context, req *pb.RegisterHeadRequest) (*pb.RegisterHeadResponse, error) {
	head := routing.HeadService{
		HeadID:        req.HeadId,
		Endpoint:      req.Endpoint,
		Status:        "active",
		Region:        req.Region,
		ModelType:     req.ModelType,
		Version:       req.Version,
		Metadata:      req.Metadata,
		LastHeartbeat: time.Now().Unix(),
	}

	if err := h.registry.Register(head); err != nil {
		return &pb.RegisterHeadResponse{
			Success: false,
			Message: "Failed to register head: " + err.Error(),
		}, err
	}

	return &pb.RegisterHeadResponse{
		Success: true,
		Message: "Head registered successfully",
	}, nil
}

// UpdateHeadStatus implements the UpdateHeadStatus RPC method
func (h *GRPCHandlers) UpdateHeadStatus(ctx context.Context, req *pb.UpdateHeadStatusRequest) (*pb.UpdateHeadStatusResponse, error) {
	err := h.registry.UpdateStatus(req.HeadId, req.Status, req.CurrentLoad, req.Timestamp)
	if err != nil {
		return &pb.UpdateHeadStatusResponse{
			Success: false,
			Message: "Failed to update head status: " + err.Error(),
		}, err
	}

	return &pb.UpdateHeadStatusResponse{
		Success: true,
		Message: "Head status updated successfully",
	}, nil
}

// GetRoutingDecision implements the GetRoutingDecision RPC method
func (h *GRPCHandlers) GetRoutingDecision(ctx context.Context, req *pb.GetRoutingDecisionRequest) (*pb.GetRoutingDecisionResponse, error) {
	decision, err := h.routingEngine.GetDecision(&routing.RoutingRequest{
		ModelType:        req.ModelType,
		RegionPreference: req.RegionPreference,
		RoutingStrategy:  req.RoutingStrategy,
		Metadata:         req.Metadata,
	})
	if err != nil {
		return &pb.GetRoutingDecisionResponse{
			HeadId:       "",
			Endpoint:     "",
			StrategyUsed: "error",
			Reason:       "Failed to get routing decision: " + err.Error(),
			Metadata:     make(map[string]string),
		}, err
	}

	return &pb.GetRoutingDecisionResponse{
		HeadId:       decision.HeadID,
		Endpoint:     decision.Endpoint,
		StrategyUsed: decision.StrategyUsed,
		Reason:       decision.Reason,
		Metadata:     decision.Metadata,
	}, nil
}

// GetAllHeads implements the GetAllHeads RPC method
func (h *GRPCHandlers) GetAllHeads(ctx context.Context, req *pb.GetAllHeadsRequest) (*pb.GetAllHeadsResponse, error) {
	heads := h.registry.GetAll()
	
	var pbHeads []*pb.HeadService
	for _, head := range heads {
		pbHeads = append(pbHeads, &pb.HeadService{
			HeadId:        head.HeadID,
			Endpoint:      head.Endpoint,
			Status:        head.Status,
			CurrentLoad:   head.CurrentLoad,
			Region:        head.Region,
			ModelType:     head.ModelType,
			Version:       head.Version,
			Metadata:      head.Metadata,
			LastHeartbeat: head.LastHeartbeat,
		})
	}

	return &pb.GetAllHeadsResponse{
		Heads: pbHeads,
	}, nil
}

// UpdateRoutingPolicy implements the UpdateRoutingPolicy RPC method
func (h *GRPCHandlers) UpdateRoutingPolicy(ctx context.Context, req *pb.UpdateRoutingPolicyRequest) (*pb.UpdateRoutingPolicyResponse, error) {
	policy := &routing.RoutingPolicy{
		DefaultStrategy:       req.DefaultStrategy,
		EnableGeoRouting:      req.EnableGeoRouting,
		EnableLoadBalancing:   req.EnableLoadBalancing,
		EnableModelSpecific:   req.EnableModelSpecific,
		StrategyConfig:        req.StrategyConfig,
		PredictionWindow:      int(req.PredictionWindow),
		LoadGrowthFactor:      req.LoadGrowthFactor,
		CapacityThreshold:     req.CapacityThreshold,
	}

	if err := h.policyMgr.Update(policy); err != nil {
		return &pb.UpdateRoutingPolicyResponse{
			Success: false,
			Message: "Failed to update routing policy: " + err.Error(),
		}, err
	}

	return &pb.UpdateRoutingPolicyResponse{
		Success: true,
		Message: "Routing policy updated successfully",
	}, nil
}

// GetRoutingPolicy implements the GetRoutingPolicy RPC method
func (h *GRPCHandlers) GetRoutingPolicy(ctx context.Context, req *pb.GetRoutingPolicyRequest) (*pb.GetRoutingPolicyResponse, error) {
	policy := h.policyMgr.Get()

	return &pb.GetRoutingPolicyResponse{
		DefaultStrategy:      policy.DefaultStrategy,
		EnableGeoRouting:     policy.EnableGeoRouting,
		EnableLoadBalancing:  policy.EnableLoadBalancing,
		EnableModelSpecific:  policy.EnableModelSpecific,
		StrategyConfig:       policy.StrategyConfig,
		PredictionWindow:     int32(policy.PredictionWindow),
		LoadGrowthFactor:     policy.LoadGrowthFactor,
		CapacityThreshold:    policy.CapacityThreshold,
	}, nil
}