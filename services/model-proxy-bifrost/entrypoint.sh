

#!/bin/sh

# Start Bifrost in the background
echo "Starting Bifrost..."
bifrost serve --app-dir /app/data --port 8100 --host 0.0.0.0 &

# Wait a bit for Bifrost to start
sleep 5

# Start gRPC adapter
echo "Starting gRPC adapter..."
/grpc-adapter

# Keep the container running
wait



