




#!/bin/bash

# Deployment script for LLM microservices
# Usage: ./deploy.sh [environment] [action]
#
# Environment: dev, staging, production
# Action: deploy, update, rollback

set -e

# Configuration
DEPLOY_DIR=$(dirname "$0")
INVENTORY_FILE="$DEPLOY_DIR/inventory.ini"
PLAYBOOK_FILE="$DEPLOY_DIR/site.yml"
ENV=${1:-dev}
ACTION=${2:-deploy}

# Environment-specific configurations
case "$ENV" in
  dev)
    export ANSIBLE_HOST_KEY_CHECKING=False
    ;;
  staging)
    export ANSIBLE_HOST_KEY_CHECKING=False
    ;;
  production)
    export ANSIBLE_HOST_KEY_CHECKING=True
    ;;
  *)
    echo "Unknown environment: $ENV"
    echo "Usage: $0 [dev|staging|production] [deploy|update|rollback]"
    exit 1
    ;;
esac

# Function to check prerequisites
check_prerequisites() {
  echo "Checking prerequisites..."

  if ! command -v ansible &> /dev/null; then
    echo "ERROR: Ansible is not installed. Please install Ansible first."
    exit 1
  fi

  if ! command -v docker &> /dev/null; then
    echo "WARNING: Docker is not installed locally (needed for building images)"
  fi

  if [ ! -f "$INVENTORY_FILE" ]; then
    echo "ERROR: Inventory file not found at $INVENTORY_FILE"
    exit 1
  fi

  if [ ! -f "$PLAYBOOK_FILE" ]; then
    echo "ERROR: Playbook file not found at $PLAYBOOK_FILE"
    exit 1
  fi
}

# Function to install Ansible roles
install_roles() {
  echo "Installing Ansible roles..."
  ansible-galaxy install -r "$DEPLOY_DIR/requirements.yml" -f
}

# Function to run the deployment
run_deployment() {
  echo "Starting deployment to $ENV environment..."

  case "$ACTION" in
    deploy)
      ansible-playbook -i "$INVENTORY_FILE" "$PLAYBOOK_FILE" \
        --extra-vars "env=$ENV action=deploy"
      ;;
    update)
      ansible-playbook -i "$INVENTORY_FILE" "$PLAYBOOK_FILE" \
        --extra-vars "env=$ENV action=update"
      ;;
    rollback)
      ansible-playbook -i "$INVENTORY_FILE" "$PLAYBOOK_FILE" \
        --extra-vars "env=$ENV action=rollback"
      ;;
    *)
      echo "Unknown action: $ACTION"
      echo "Usage: $0 [dev|staging|production] [deploy|update|rollback]"
      exit 1
      ;;
  esac
}

# Main execution
echo "LLM Microservices Deployment Script"
echo "Environment: $ENV"
echo "Action: $ACTION"
echo ""

check_prerequisites
install_roles
run_deployment

echo ""
echo "Deployment completed successfully!"


