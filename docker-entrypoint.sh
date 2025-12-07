

#!/bin/bash
set -e

# Start routing-service
echo "Starting routing-service..."
/usr/local/bin/routing-service &

# Start secrets-service
echo "Starting secrets-service..."
/usr/local/bin/secrets-service &

# Start network-config-admin
echo "Starting network-config-admin..."
/usr/local/bin/network-config-admin &

# Start telegram-bot
echo "Starting telegram-bot..."
cd /app/telegram-bot
npm start &

# Start monitoring tools (if needed)
# echo "Starting monitoring tools..."
# prometheus --config.file=/app/monitoring/prometheus/prometheus.yml &

echo "All services started successfully!"

# Wait for all background processes
wait -n

