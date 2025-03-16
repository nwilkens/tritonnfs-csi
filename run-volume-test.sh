#!/bin/bash
set -e

# Help text
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
  echo "Usage: ./run-volume-test.sh [options]"
  echo ""
  echo "This script runs a test of Triton volume operations using environment variables for authentication."
  echo ""
  echo "Environment variables (all required):"
  echo "  TRITON_CLOUD_API         - Triton CloudAPI endpoint (e.g., https://us-central-1.api.mnx.io)"
  echo "  TRITON_ACCOUNT_ID        - Your Triton account ID/name"
  echo "  TRITON_KEY_ID            - Your SSH key fingerprint"
  echo "  TRITON_PRIVATE_KEY_FILE  - Path to your SSH private key file"
  echo ""
  echo "Options:"
  echo "  -h, --help               - Show this help message"
  exit 0
fi

# Check if environment variables are set, prompt if not
if [ -z "$TRITON_CLOUD_API" ]; then
  read -p "Enter Triton CloudAPI endpoint (e.g., https://us-central-1.api.mnx.io): " TRITON_CLOUD_API
  export TRITON_CLOUD_API
fi

if [ -z "$TRITON_ACCOUNT_ID" ]; then
  read -p "Enter your Triton account ID/name: " TRITON_ACCOUNT_ID
  export TRITON_ACCOUNT_ID
fi

if [ -z "$TRITON_KEY_ID" ]; then
  read -p "Enter your SSH key fingerprint: " TRITON_KEY_ID
  export TRITON_KEY_ID
fi

if [ -z "$TRITON_PRIVATE_KEY_FILE" ]; then
  read -p "Enter path to your SSH private key file: " TRITON_PRIVATE_KEY_FILE
  export TRITON_PRIVATE_KEY_FILE
fi

# Validate that the private key file exists
if [ ! -f "$TRITON_PRIVATE_KEY_FILE" ]; then
  echo "ERROR: Private key file not found at $TRITON_PRIVATE_KEY_FILE"
  exit 1
fi

echo "Building and running the volume test..."
echo ""

# Compile and run the test program
go run test-volume-ops.go