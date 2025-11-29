


#!/bin/bash

# Integration test script for Secret Service

echo "Starting integration test for Secret Service..."

# Test environment variables
export VAULT_ADDR="http://localhost:8200"
export VAULT_TOKEN="root"
export ADMIN_KEY="test-admin-key"

# Start the service in the background
echo "Starting secret service..."
./secret-service &

SERVICE_PID=$!
sleep 5  # Wait for service to start

# Test health endpoint
echo "Testing health endpoint..."
HEALTH_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/health)
if [ "$HEALTH_RESPONSE" -eq 200 ]; then
    echo "✓ Health endpoint returned 200 OK"
else
    echo "✗ Health endpoint failed with status $HEALTH_RESPONSE"
    kill $SERVICE_PID
    exit 1
fi

# Test metrics endpoint
echo "Testing metrics endpoint..."
METRICS_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8082/metrics)
if [ "$METRICS_RESPONSE" -eq 200 ]; then
    echo "✓ Metrics endpoint returned 200 OK"
else
    echo "✗ Metrics endpoint failed with status $METRICS_RESPONSE"
    kill $SERVICE_PID
    exit 1
fi

# Test admin API with invalid key
echo "Testing admin API with invalid key..."
INVALID_KEY_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Admin-Key: invalid-key" http://localhost:8082/admin/api/secrets)
if [ "$INVALID_KEY_RESPONSE" -eq 403 ]; then
    echo "✓ Admin API correctly rejected invalid key"
else
    echo "✗ Admin API failed to reject invalid key, returned $INVALID_KEY_RESPONSE"
    kill $SERVICE_PID
    exit 1
fi

# Test admin API with valid key
echo "Testing admin API with valid key..."
VALID_KEY_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" -H "X-Admin-Key: test-admin-key" http://localhost:8082/admin/api/secrets)
if [ "$VALID_KEY_RESPONSE" -eq 200 ]; then
    echo "✓ Admin API accepted valid key"
else
    echo "✗ Admin API failed with valid key, returned $VALID_KEY_RESPONSE"
    kill $SERVICE_PID
    exit 1
fi

# Clean up
echo "Stopping service..."
kill $SERVICE_PID

echo "All integration tests passed!"


