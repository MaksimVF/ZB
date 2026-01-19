

package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/redis/go-redis/v9"
)

type HealthChecker struct {
	redisClient *redis.Client
}

func NewHealthChecker(redisClient *redis.Client) *HealthChecker {
	return &HealthChecker{
		redisClient: redisClient,
	}
}

func (h *HealthChecker) CheckRedisHealth() bool {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := h.redisClient.Ping(ctx).Result()
	return err == nil
}

func (h *HealthChecker) HealthCheckHandler(w http.ResponseWriter, r *http.Request) {
	redisHealthy := h.CheckRedisHealth()

	if !redisHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"status": "unhealthy", "redis": "unavailable"}`))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "healthy", "redis": "available"}`))
}

// Metrics setup
var (
	chargeRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_charge_requests_total",
			Help: "Total number of charge requests",
		},
		[]string{"success"},
	)

	reserveRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_reserve_requests_total",
			Help: "Total number of reserve requests",
		},
		[]string{"success"},
	)

	commitRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_commit_requests_total",
			Help: "Total number of commit requests",
		},
		[]string{"success"},
	)

	balanceRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_balance_requests_total",
			Help: "Total number of balance requests",
		},
		[]string{"success"},
	)

	adjustBalanceRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "billing_adjust_balance_requests_total",
			Help: "Total number of adjust balance requests",
		},
		[]string{"success"},
	)

	processingTime = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "billing_processing_time_seconds",
			Help:    "Processing time for billing operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

func initMetrics() {
	prometheus.MustRegister(chargeRequests)
	prometheus.MustRegister(reserveRequests)
	prometheus.MustRegister(commitRequests)
	prometheus.MustRegister(balanceRequests)
	prometheus.MustRegister(adjustBalanceRequests)
	prometheus.MustRegister(processingTime)
}

func StartMetricsServer(port int) {
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "ok"}`))
	})

	go func() {
		log.Printf("Metrics server starting on port %d", port)
		if err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil); err != nil {
			log.Printf("Metrics server failed: %v", err)
		}
	}()
}

