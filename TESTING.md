# Testing TritonNFS CSI Driver

This document outlines the testing approaches for the TritonNFS CSI driver.

## Testing Authentication and Volume Operations

The repository includes a Go test program (`test-volume-ops.go`) that verifies the core API interactions with Triton:

1. Authentication with Triton CloudAPI
2. Listing volumes
3. Creating a new volume
4. Retrieving a specific volume
5. Deleting a volume

This test is designed to validate the basic functionality of the triton-go client used by the CSI driver.

### Running the Volume Operations Test

The easiest way to run the test is using the provided shell script:

```bash
./run-volume-test.sh
```

The script will prompt for the necessary Triton credentials if they are not already set as environment variables.

Alternatively, you can set the environment variables yourself and run the test directly:

```bash
export TRITON_CLOUD_API="https://us-central-1.api.mnx.io"  # Your Triton CloudAPI endpoint
export TRITON_ACCOUNT_ID="your-account-name"               # Your Triton account ID/name
export TRITON_KEY_ID="xx:xx:xx:..."                        # Your SSH key fingerprint
export TRITON_PRIVATE_KEY_FILE="/path/to/your/key.pem"     # Path to your SSH private key file

go run test-volume-ops.go
```

### Environment Variables

The test and driver support the following environment variables:

| Variable | Description | Example |
|----------|-------------|---------|
| `TRITON_CLOUD_API` | Triton CloudAPI endpoint | `https://us-central-1.api.mnx.io` |
| `TRITON_ACCOUNT_ID` | Your Triton account ID/name | `your-account-name` |
| `TRITON_KEY_ID` | Your SSH key fingerprint | `xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx:xx` |
| `TRITON_PRIVATE_KEY_FILE` | Path to your SSH private key file | `/path/to/your/key.pem` |

## Kubernetes Integration Testing

After validating the basic volume operations, you can test the CSI driver in a Kubernetes environment using the example YAML files provided in the `examples/` directory.

### Prerequisites

1. A running Kubernetes cluster
2. Triton credentials configured in a Kubernetes Secret

### Testing Steps

1. Deploy the CSI driver using the YAML files in the `examples/` directory
2. Create a PersistentVolumeClaim (PVC) using the Triton storage class
3. Deploy a Pod that mounts the PVC
4. Verify that the volume is correctly provisioned, mounted, and usable

Refer to `examples/INSTALLATION.md` for detailed deployment instructions.

## Troubleshooting

If you encounter issues during testing:

1. Check that your Triton credentials are correct
2. Verify that your private key file is in the correct format (PEM)
3. Ensure that the Triton CloudAPI endpoint is accessible
4. Check the logs of the CSI driver pods
5. If using the triton CLI, verify that you can create and manage volumes directly using the CLI