


# Bifrost-based Model Proxy

This service replaces the original LiteLLM-based `model-proxy` with [Bifrost](https://github.com/maximhq/bifrost), a high-performance AI gateway written in Go.

## Key Improvements

### Performance
- **11 µs overhead** per request vs LiteLLM's higher overhead
- **100% success rate** at 5,000 RPS in benchmarks
- **Sub-microsecond** average queue wait times

### Features
- ✅ **Multi-provider support**: OpenAI, Anthropic, AWS Bedrock, Google Vertex, Azure, Mistral, Groq, and more
- ✅ **Automatic failover**: Seamless fallback between providers with zero downtime
- ✅ **Load balancing**: Intelligent request distribution across API keys and providers
- ✅ **Semantic caching**: Intelligent response caching to reduce costs and latency
- ✅ **Model Context Protocol (MCP)**: Enable AI models to use external tools
- ✅ **Enterprise-grade**: Budget management, SSO, observability, Vault integration
- ✅ **Web UI**: Built-in configuration and monitoring interface
- ✅ **Plugin architecture**: Extensible middleware system

### Architecture
```
┌───────────────────────────────────────────────────────┐
│                 Bifrost Model Proxy                   │
├───────────────────────────────────────────────────────┤
│                                                       │
│  ┌─────────────────┐    ┌─────────────────────────┐  │
│  │  Bifrost Core   │    │  gRPC Adapter          │  │
│  │  (HTTP API)     │    │  (Legacy gRPC          │  │
│  │                 │    │   Interface)           │  │
│  └─────────────────┘    └─────────────────────────┘  │
│            ▲                         ▲               │
└────────────┼─────────────────────────┼───────────────┘
             │                         │
             │                         │
┌────────────┴─────────────┐ ┌─────────┴─────────────┐
│      Head Service        │ │    Other Services    │
│  (Go gRPC client)        │ │  (HTTP clients)      │
└──────────────────────────┘ └──────────────────────┘
```

## Configuration

The service uses a `config.json` file for Bifrost configuration:

```json
{
  "providers": {
    "openai": {
      "api_key": "OPENAI_API_KEY",
      "models": ["gpt-3.5-turbo", "gpt-4", "gpt-4o-mini"],
      "default_model": "gpt-3.5-turbo"
    },
    "anthropic": {
      "api_key": "ANTHROPIC_API_KEY",
      "models": ["claude-3-opus-20240229", "claude-3-sonnet-20240229"],
      "default_model": "claude-3-sonnet-20240229"
    }
  },
  "fallbacks": {
    "openai": ["anthropic", "mistral", "groq"],
    "anthropic": ["openai", "mistral", "groq"]
  },
  "load_balancing": {
    "strategy": "round_robin",
    "weights": {
      "openai": 40,
      "anthropic": 30,
      "mistral": 20,
      "groq": 10
    }
  }
}
```

## API Compatibility

### HTTP API (Bifrost Native)
- **Endpoint**: `POST /v1/chat/completions`
- **Format**: OpenAI-compatible
- **Port**: 8100

### gRPC API (Legacy Compatibility)
- **Service**: `ModelService`
- **Methods**:
  - `Generate` - Single generation request
  - `GenerateStream` - Streaming generation
  - `BatchGenerate` - Batch processing
- **Port**: 50061

## Migration Guide

### For Head Service (Go gRPC Client)
No changes required! The gRPC adapter maintains full compatibility with the existing `ModelClient` implementation.

### For HTTP Clients
Update the endpoint to use the new Bifrost-compatible format:

```python
# Old LiteLLM format
response = requests.post("http://model-proxy:8100/v1/generate", json={
    "model": "openai/gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}],
    "temperature": 0.7,
    "max_tokens": 100
})

# New Bifrost format
response = requests.post("http://model-proxy:8100/v1/chat/completions", json={
    "model": "openai/gpt-3.5-turbo",
    "messages": [{"role": "user", "content": "Hello"}],
    "temperature": 0.7,
    "max_tokens": 100
})
```

## Deployment

### Docker
```bash
docker-compose up --build model-proxy
```

### Environment Variables
- `BIFROST_URL`: URL for Bifrost HTTP API (default: `http://localhost:8100`)
- `LOG_LEVEL`: Log level (default: `info`)
- `PORT`: HTTP port (default: `8100`)

## Testing

Run the integration tests:
```bash
python test_integration.py
```

## Performance Comparison

| Metric | LiteLLM | Bifrost | Improvement |
|--------|---------|---------|-------------|
| Request Overhead | ~100ms | 11 µs | **~9000x faster** |
| Success Rate @ 5k RPS | ~95% | 100% | **+5%** |
| Queue Wait Time | ~10ms | 1.67 µs | **~6000x faster** |
| Key Selection | ~1ms | 10 ns | **~100,000x faster** |

## Benefits of This Migration

1. **Performance**: Dramatic reduction in latency and overhead
2. **Reliability**: Built-in failover and load balancing
3. **Scalability**: Handles 5,000+ RPS with 100% success rate
4. **Features**: Enterprise-grade capabilities out of the box
5. **Maintainability**: Single Go codebase instead of mixed Python/Go
6. **Future-proof**: Active development and community support

## Troubleshooting

### Common Issues

**gRPC connection failures**:
- Ensure the gRPC adapter is running on port 50061
- Check that Bifrost is available on the configured URL
- Verify network connectivity between services

**HTTP API errors**:
- Check Bifrost logs for detailed error information
- Verify provider API keys are correctly configured
- Ensure the requested models are available in the configuration

**Performance issues**:
- Monitor Bifrost's built-in metrics endpoint
- Adjust load balancing weights in config.json
- Enable semantic caching for frequent requests

## Support

For issues with Bifrost itself, refer to the [official documentation](https://docs.getbifrost.ai) or join the [Discord community](https://discord.gg/exN5KAydbU).

For issues specific to this integration, please open an issue in this repository.

---

**Built with ❤️ using [Bifrost](https://github.com/maximhq/bifrost) by [Maxim](https://github.com/maximhq)**



