package config
import "os"
type Config struct {
    GRPCAddr string
    MetricsPort int
    ModelProxyAddr string
}
func Load() *Config {
    return &Config{ GRPCAddr: ":50055", MetricsPort:9001, ModelProxyAddr: os.Getenv("MODEL_ADDR") }
}
