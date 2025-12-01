




package webhook

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
)

// WebhookConfig holds webhook configuration
type WebhookConfig struct {
    URL            string
    Timeout         time.Duration
    MaxRetries      int
    RetryDelay      time.Duration
    Enabled         bool
}

// WebhookClient handles webhook notifications
type WebhookClient struct {
    config WebhookConfig
    client *http.Client
    mu     sync.Mutex
}

// WebhookPayload represents the payload sent to webhooks
type WebhookPayload struct {
    EventType string      `json:"event_type"`
    Timestamp time.Time    `json:"timestamp"`
    Data       interface{} `json:"data"`
}

// NewWebhookClient creates a new webhook client
func NewWebhookClient(config WebhookConfig) *WebhookClient {
    return &WebhookClient{
        config: config,
        client: &http.Client{
            Timeout: config.Timeout,
        },
    }
}

// SendWebhook sends a webhook notification
func (w *WebhookClient) SendWebhook(ctx context.Context, eventType string, data interface{}) error {
    if !w.config.Enabled {
        return nil
    }

    // Start a span for the webhook operation
    tracer := otel.GetTracerProvider().Tracer("head-go")
    ctx, span := tracer.Start(ctx, "Webhook.SendWebhook")
    defer span.End()

    span.SetAttributes(
        attribute.String("event_type", eventType),
        attribute.String("webhook_url", w.config.URL),
    )

    payload := WebhookPayload{
        EventType: eventType,
        Timestamp: time.Now(),
        Data:      data,
    }

    body, err := json.Marshal(payload)
    if err != nil {
        span.SetStatus(trace.StatusCodeError, "failed to marshal payload")
        span.RecordError(err)
        return fmt.Errorf("failed to marshal webhook payload: %w", err)
    }

    req, err := http.NewRequestWithContext(ctx, "POST", w.config.URL, bytes.NewReader(body))
    if err != nil {
        span.SetStatus(trace.StatusCodeError, "failed to create request")
        span.RecordError(err)
        return fmt.Errorf("failed to create webhook request: %w", err)
    }

    req.Header.Set("Content-Type", "application/json")

    // Execute with retry logic
    var lastErr error
    for attempt := 0; attempt <= w.config.MaxRetries; attempt++ {
        resp, err := w.client.Do(req)
        if err != nil {
            lastErr = err
            span.RecordError(err)

            // If this is the last attempt, break
            if attempt == w.config.MaxRetries {
                break
            }

            // Wait before retrying
            time.Sleep(w.config.RetryDelay)
            continue
        }

        // Check response status
        if resp.StatusCode >= 200 && resp.StatusCode < 300 {
            // Success
            resp.Body.Close()
            return nil
        }

        // Error response
        lastErr = fmt.Errorf("webhook returned status %d", resp.StatusCode)
        resp.Body.Close()

        // If this is the last attempt, break
        if attempt == w.config.MaxRetries {
            break
        }

        // Wait before retrying
        time.Sleep(w.config.RetryDelay)
    }

    span.SetStatus(trace.StatusCodeError, "webhook failed")
    span.RecordError(lastErr)
    return fmt.Errorf("webhook failed after %d attempts: %w", w.config.MaxRetries+1, lastErr)
}

// SendAsyncWebhook sends a webhook asynchronously
func (w *WebhookClient) SendAsyncWebhook(eventType string, data interface{}) {
    go func() {
        ctx := context.Background()
        if err := w.SendWebhook(ctx, eventType, data); err != nil {
            // Log the error (in a real implementation, this would use proper logging)
            fmt.Printf("Async webhook failed: %v\n", err)
        }
    }()
}

// Enable enables webhook notifications
func (w *WebhookClient) Enable() {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.config.Enabled = true
}

// Disable disables webhook notifications
func (w *WebhookClient) Disable() {
    w.mu.Lock()
    defer w.mu.Unlock()
    w.config.Enabled = false
}

// IsEnabled checks if webhooks are enabled
func (w *WebhookClient) IsEnabled() bool {
    w.mu.Lock()
    defer w.mu.Unlock()
    return w.config.Enabled
}




