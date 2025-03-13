# TritonNFS CSI Driver Deployment Guide

This directory contains example configurations for deploying the TritonNFS CSI driver in a Kubernetes cluster. These examples show how to set up the CSI driver to provision NFS volumes from Triton.

## Prerequisites

- A Kubernetes cluster (v1.19+)
- Triton account with access credentials
- `kubectl` installed and configured to access your cluster

## Components

The deployment consists of the following components:

1. **CSIDriver object** - Registers the CSI driver with Kubernetes
2. **Controller deployment** - Runs the provisioner, attacher, and resizer sidecars along with the TritonNFS CSI plugin
3. **Node DaemonSet** - Runs the node plugin on each node to handle mounting volumes
4. **RBAC** - Service accounts, roles, and role bindings required by the CSI driver
5. **StorageClass** - Configures how volumes are provisioned
6. **Secret** - Stores Triton authentication credentials

## Deployment Steps

### 1. Create Triton credentials secret

Edit the `secret.yaml` file to include your Triton account credentials:

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: triton-creds
  namespace: kube-system
type: Opaque
stringData:
  cloudapi: "https://your-triton-endpoint.api.joyent.com"  # Your Triton API endpoint
  account-id: "your-account-id"                            # Your Triton account ID
  key-id: "your-key-id"                                    # Your SSH key fingerprint
  key.pem: |
    -----BEGIN RSA PRIVATE KEY-----
    Your private key contents here
    -----END RSA PRIVATE KEY-----
```

Apply the secret:

```bash
kubectl apply -f secret.yaml
```

### 2. Deploy the CSI Driver components

```bash
kubectl apply -f csidriver.yaml
kubectl apply -f rbac.yaml
kubectl apply -f controller.yaml
kubectl apply -f node.yaml
kubectl apply -f storageclass.yaml
```

### 3. Verify the deployment

Check that the controller pod is running:

```bash
kubectl get pods -n kube-system -l app=tritonnfs-csi-controller
```

Check that the node pods are running:

```bash
kubectl get pods -n kube-system -l app=tritonnfs-csi-node
```

Verify the CSI driver registration:

```bash
kubectl get csidriver tritonnfs.csi.triton.com
```

### 4. Creating volumes

Create a PersistentVolumeClaim that uses the StorageClass:

```bash
kubectl apply -f pvc.yaml
```

Verify the PVC is bound:

```bash
kubectl get pvc
```

### 5. Using volumes in pods

Create a pod that uses the PVC:

```bash
kubectl apply -f pod.yaml
```

## Configuration Options

### StorageClass Parameters

The StorageClass supports the following parameters:

- `networks`: Comma-separated list of Triton network IDs to connect the NFS volume to
- Tags can be added with the prefix `tag-`, for example: `tag-environment: production`

## Troubleshooting

Check the logs of the controller and node pods:

```bash
kubectl logs -n kube-system -l app=tritonnfs-csi-controller -c tritonnfs-csi-plugin
kubectl logs -n kube-system -l app=tritonnfs-csi-node -c tritonnfs-csi-plugin
```

## Additional Information

For more details about the TritonNFS CSI driver, refer to the main project documentation.

## Features Documentation

- [Volume Expansion](./volume-expansion.md) - Guide to expanding the size of volumes