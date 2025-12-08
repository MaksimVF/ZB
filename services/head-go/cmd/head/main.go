package main
import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    "github.com/yourorg/head/internal/config"
    "github.com/yourorg/head/internal/metrics"
    "github.com/yourorg/head/internal/server"
)
func main(){
    // Load static configuration
    cfg := config.Load()

    // Initialize network config manager
    networkConfigManager := config.NewNetworkConfigManager("redis:6379")
    err := networkConfigManager.LoadConfig()
    if err != nil {
        log.Printf("Failed to load network config: %v", err)
    }

    // Start auto-reload for network config
    networkConfigManager.StartAutoReload(10 * time.Second)

    go metrics.Start(cfg.MetricsPort)
    srv := server.New(cfg, networkConfigManager)
    errCh := make(chan error,1)
    go func(){ errCh <- srv.Run() }()
    sig := make(chan os.Signal,1)
    signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
    select {
    case s := <-sig:
        log.Printf("signal %v, shutting", s)
    case e := <-errCh:
        log.Printf("server error %v", e)
    }
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    _ = srv.Shutdown(ctx)
}
