package strategies

import (
	routing "github.com/MaksimVF/ZB/services/routing-service/routing"
)

// GeoPreferredStrategy selects a head in the preferred region first
type GeoPreferredStrategy struct{}

// NewGeoPreferredStrategy creates a new geo-preferred strategy
func NewGeoPreferredStrategy() *GeoPreferredStrategy {
	return &GeoPreferredStrategy{}
}

// Name returns the strategy name
func (s *GeoPreferredStrategy) Name() string {
	return "geo_preferred"
}

// SelectHead selects a head in the preferred region, falls back to round-robin
func (s *GeoPreferredStrategy) SelectHead(heads []routing.HeadService, req *routing.RoutingRequest) *routing.HeadService {
	if len(heads) == 0 {
		return nil
	}

	// First try to find a head in the preferred region
	for _, head := range heads {
		if head.Region == req.RegionPreference {
			return &head
		}
	}

	// If no head in preferred region, fall back to round-robin (select first)
	return &heads[0]
}