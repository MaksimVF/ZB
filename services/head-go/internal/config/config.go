package config

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	GRPCAddr string
	MetricsPort int
	CacheAddr string
	CacheEnabled bool
	CacheTTL int
	ProviderKeys map[string]string
	Fallbacks map[string][]string
	MTLS bool
}

func Load() *Config {
	_ = godotenv.Load()
	c := &Config{
		GRPCAddr: ":50055",
		MetricsPort: 9001,
		CacheAddr: "cache:50053",
		CacheEnabled: false,
		CacheTTL: 300,
		ProviderKeys: map[string]string{},
		Fallbacks: map[string][]string{},
	}
	if v:=os.Getenv("CHAT_ADDR"); v!="" { c.GRPCAddr = v }
	if v:=os.Getenv("METRICS_PORT"); v!="" { if p,err:=strconv.Atoi(v);err==nil{c.MetricsPort=p} }
	if v:=os.Getenv("CACHE_ENABLED"); strings.ToLower(v)=="true" { c.CacheEnabled=true }
	parseJSON := func(env string, out interface{}) {
		raw := os.Getenv(env); if raw=="" { return }
		s := strings.TrimSpace(raw)
		if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) || (strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
			s = s[1:len(s)-1]
		}
		s = strings.ReplaceAll(s, "\\n", "")
		_ = json.Unmarshal([]byte(s), out)
	}
	parseJSON("PROVIDER_KEYS",&c.ProviderKeys)
	parseJSON("FALLBACKS",&c.Fallbacks)
	if os.Getenv("MTLS_ENABLED")=="1" { c.MTLS=true }
	return c
}
