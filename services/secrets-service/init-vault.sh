





#!/bin/bash

# Wait for Vault to be ready
echo "Waiting for Vault to be ready..."
while ! curl -s http://localhost:8200/v1/sys/health > /dev/null; do
  sleep 1
done

echo "Vault is ready!"

# Set environment variables
export VAULT_ADDR=http://127.0.0.1:8200
export VAULT_TOKEN=root

# Enable KV v2
echo "Enabling KV v2..."
vault secrets enable -path=secret kv-v2

# Create policy for gateway
echo "Creating gateway policy..."
vault policy write gateway-policy - <<EOF
path "secret/data/llm/*" {
  capabilities = ["read"]
}
path "secret/data/providers/*" {
  capabilities = ["read"]
}
EOF

# Create token for gateway (TTL 7 days, renewable)
echo "Creating gateway token..."
vault token create -policy=gateway-policy -ttl=168h -renewable=true

# Create token for secret service (full access)
echo "Creating secret service token..."
vault token create -policy=system -ttl=0

echo "Vault initialization complete!"













