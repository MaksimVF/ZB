
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"go.uber.org/zap"
)

// TailscaleStatus represents the status of a Tailscale connection
type TailscaleStatus struct {
	Self struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
		Hostname    string   `json:"HostName"`
	} `json:"Self"`
	Peer []struct {
		TailscaleIPs []string `json:"TailscaleIPs"`
		HostName    string   `json:"HostName"`
	} `json:"Peer"`
}

// TailscaleManager handles Tailscale operations
type TailscaleManager struct {
	logger *zap.Logger
}

// NewTailscaleManager creates a new Tailscale manager
func NewTailscaleManager(logger *zap.Logger) *TailscaleManager {
	return &TailscaleManager{logger: logger}
}

// ConfigureTailscale configures Tailscale with the given parameters
func (m *TailscaleManager) ConfigureTailscale(authKey, hostname, advertiseRoutes string) error {
	// Check if Tailscale is installed
	if !m.isTailscaleInstalled() {
		return fmt.Errorf("Tailscale is not installed")
	}

	// Check if we're running in Docker and use appropriate command
	useDocker := m.isRunningInDocker()

	// Authenticate with Tailscale
	if authKey != "" {
		if useDocker {
			if err := m.runTailscaleDockerCommand("up", "--authkey", authKey, "--hostname", hostname); err != nil {
				return fmt.Errorf("failed to authenticate with Tailscale: %w", err)
			}
		} else {
			if err := m.runTailscaleCommand("up", "--authkey", authKey, "--hostname", hostname); err != nil {
				return fmt.Errorf("failed to authenticate with Tailscale: %w", err)
			}
		}
	}

	// Configure advertise routes if specified
	if advertiseRoutes != "" {
		if useDocker {
			if err := m.runTailscaleDockerCommand("set", "--advertise-routes", advertiseRoutes); err != nil {
				return fmt.Errorf("failed to set advertise routes: %w", err)
			}
		} else {
			if err := m.runTailscaleCommand("set", "--advertise-routes", advertiseRoutes); err != nil {
				return fmt.Errorf("failed to set advertise routes: %w", err)
			}
		}
	}

	return nil
}

// GetTailscaleStatus gets the current Tailscale status
func (m *TailscaleManager) GetTailscaleStatus() (*TailscaleStatus, error) {
	var statusOutput []byte
	var err error

	if m.isRunningInDocker() {
		statusOutput, err = m.runTailscaleDockerCommandWithOutput("status", "--json")
	} else {
		statusOutput, err = m.runTailscaleCommandWithOutput("status", "--json")
	}

	if err != nil {
		return nil, fmt.Errorf("failed to get Tailscale status: %w", err)
	}

	var status TailscaleStatus
	if err := json.Unmarshal(statusOutput, &status); err != nil {
		return nil, fmt.Errorf("failed to parse Tailscale status: %w", err)
	}

	return &status, nil
}

// isTailscaleInstalled checks if Tailscale is installed on the system
func (m *TailscaleManager) isTailscaleInstalled() bool {
	_, err := exec.LookPath("tailscale")
	return err == nil
}

// isRunningInDocker checks if we're running inside a Docker container
func (m *TailscaleManager) isRunningInDocker() bool {
	// Check for Docker-specific files or environment
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroups
	data, err := os.ReadFile("/proc/self/cgroup")
	if err == nil {
		return strings.Contains(string(data), "docker") || strings.Contains(string(data), "kubepods")
	}

	return false
}

// runTailscaleCommand runs a Tailscale command
func (m *TailscaleManager) runTailscaleCommand(args ...string) error {
	cmd := exec.Command("tailscale", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Tailscale command failed", zap.String("command", strings.Join(args, " ")), zap.String("output", string(output)))
		return err
	}
	return nil
}

// runTailscaleCommandWithOutput runs a Tailscale command and returns the output
func (m *TailscaleManager) runTailscaleCommandWithOutput(args ...string) ([]byte, error) {
	cmd := exec.Command("tailscale", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Tailscale command failed", zap.String("command", strings.Join(args, " ")), zap.String("output", string(output)))
		return nil, err
	}
	return output, nil
}

// runTailscaleDockerCommand runs a Tailscale command using the Docker device
func (m *TailscaleManager) runTailscaleDockerCommand(args ...string) error {
	// Use the Tailscale Docker device if available
	dockerArgs := []string{"exec", "tailscale", "tailscale"}
	dockerArgs = append(dockerArgs, args...)
	cmd := exec.Command("docker", dockerArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Tailscale Docker command failed", zap.String("command", strings.Join(args, " ")), zap.String("output", string(output)))
		return err
	}
	return nil
}

// runTailscaleDockerCommandWithOutput runs a Tailscale command using the Docker device and returns the output
func (m *TailscaleManager) runTailscaleDockerCommandWithOutput(args ...string) ([]byte, error) {
	dockerArgs := []string{"exec", "tailscale", "tailscale"}
	dockerArgs = append(dockerArgs, args...)
	cmd := exec.Command("docker", dockerArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		m.logger.Error("Tailscale Docker command failed", zap.String("command", strings.Join(args, " ")), zap.String("output", string(output)))
		return nil, err
	}
	return output, nil
}

// ApplyNetworkConfig applies the Tailscale configuration from NetworkConfig
func (m *TailscaleManager) ApplyNetworkConfig(config NetworkConfig) error {
	if config.NetworkMode != "tailscale" {
		return nil // Skip if not using Tailscale mode
	}

	m.logger.Info("Applying Tailscale configuration",
		zap.String("mode", config.NetworkMode),
		zap.String("hostname", config.TailscaleHostname))

	return m.ConfigureTailscale(
		config.TailscaleAuthKey,
		config.TailscaleHostname,
		config.TailscaleAdvertiseRoutes,
	)
}

