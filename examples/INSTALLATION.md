# TritonNFS CSI Driver Installation Guide

This guide provides step-by-step instructions for deploying the TritonNFS CSI driver on a Kubernetes cluster.

## Prerequisites

Before installing the TritonNFS CSI driver, ensure you have:

1. A running Kubernetes cluster (v1.19+)
2. `kubectl` command-line tool installed and configured
3. Triton account credentials:
   - Triton Cloud API endpoint
   - Account ID
   - SSH key ID (fingerprint)
   - SSH private key

## Installation Steps

### 1. Clone this repository

```bash
git clone https://github.com/joyent/tritonnfs-csi.git
cd tritonnfs-csi/examples
```

### 2. Create the Triton credentials secret

Edit the `secret.yaml` file with your Triton credentials:

```bash
# Open the file in your favorite editor
vi secret.yaml
```

Replace the placeholder values:
- `cloudapi`: Your Triton Cloud API endpoint (e.g., "https://us-east-1.api.joyent.com")
- `account-id`: Your Triton account ID
- `key-id`: Your SSH key fingerprint
- `key.pem`: Your SSH private key contents

Apply the secret:

```bash
kubectl apply -f secret.yaml
```

### 3. Deploy the CSI driver components

Apply all the configuration files in the following order:

```bash
# Create the CSI driver registration
kubectl apply -f csidriver.yaml

# Create RBAC resources (service accounts, roles, role bindings)
kubectl apply -f rbac.yaml

# Deploy the CSI controller
kubectl apply -f controller.yaml

# Deploy the node plugin as a DaemonSet
kubectl apply -f node.yaml

# Create the StorageClass
kubectl apply -f storageclass.yaml
```

Alternatively, you can apply all resources at once:

```bash
kubectl apply -f csidriver.yaml -f rbac.yaml -f controller.yaml -f node.yaml -f storageclass.yaml
```

### 4. Verify the installation

Check that the controller pod is running:

```bash
kubectl get pods -n kube-system -l app=tritonnfs-csi-controller
```

Verify that the node plugin pods are running on each node:

```bash
kubectl get pods -n kube-system -l app=tritonnfs-csi-node
```

Confirm the CSI driver is registered:

```bash
kubectl get csidriver tritonnfs.csi.triton.com
```

Check the StorageClass:

```bash
kubectl get storageclass tritonnfs
```

### 5. Test the driver

Create a test PVC:

```bash
kubectl apply -f pvc.yaml
```

Verify that the PVC is bound:

```bash
kubectl get pvc
```

Create a test pod that uses the PVC:

```bash
kubectl apply -f pod.yaml
```

Verify the pod is running and can access the volume:

```bash
kubectl get pod test-pod
kubectl exec -it test-pod -- ls -la /data
```

## Customization

### StorageClass Parameters

You can customize the StorageClass by editing `storageclass.yaml` before applying it. The TritonNFS CSI driver supports these parameters:

- `networks`: Comma-separated list of Triton network IDs to connect the NFS volume to
- Tags can be added with the prefix `tag-`, for example: `tag-environment: production`

## Troubleshooting

If you encounter issues:

1. Check the controller pod logs:

```bash
kubectl logs -n kube-system -l app=tritonnfs-csi-controller -c tritonnfs-csi-plugin
```

2. Check the node plugin logs:

```bash
kubectl logs -n kube-system -l app=tritonnfs-csi-node -c tritonnfs-csi-plugin
```

3. Verify the secret was created correctly:

```bash
kubectl describe secret -n kube-system triton-creds
```

4. Check that all pods have access to the secret:

```bash
kubectl get events -n kube-system
```

## Uninstallation

To remove the TritonNFS CSI driver:

```bash
kubectl delete -f pod.yaml -f pvc.yaml  # Delete any resources using the driver
kubectl delete -f storageclass.yaml -f node.yaml -f controller.yaml -f rbac.yaml -f csidriver.yaml -f secret.yaml
```

Note: This will delete all resources associated with the driver, including any PVCs that use the TritonNFS StorageClass. Make sure to back up any important data before uninstalling.