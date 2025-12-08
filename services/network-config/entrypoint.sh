


#!/bin/sh

# Tailscale Docker entrypoint script

set -e

# Start Tailscale
if [ -n "$TAILSCALE_AUTH_KEY" ]; then
  echo "Starting Tailscale with auth key..."
  tailscale up --authkey="$TAILSCALE_AUTH_KEY" --hostname="$TAILSCALE_HOSTNAME"

  # Configure advertise routes if specified
  if [ -n "$TAILSCALE_ADVERTISE_ROUTES" ]; then
    echo "Configuring advertise routes: $TAILSCALE_ADVERTISE_ROUTES"
    tailscale set --advertise-routes="$TAILSCALE_ADVERTISE_ROUTES"
  fi
else
  echo "No auth key provided, starting Tailscale without authentication..."
  tailscale up --hostname="$TAILSCALE_HOSTNAME"
fi

# Keep the container running
echo "Tailscale started, keeping container alive..."
trap 'echo "Shutting down..."; tailscale down; exit 0' TERM INT
tail -f /dev/null & wait



