# Triton NFS CSI Driver

A Container Storage Interface (CSI) driver for Triton NFS volumes. This driver allows Kubernetes clusters to dynamically provision NFS volumes in Triton DataCenter.

## Features

- Dynamic provisioning of Triton NFS volumes
- Support for multiple networks
- Volume tagging
- Volume expansion (resize)
- Mounting NFS volumes to pods

## Prerequisites

- Kubernetes cluster (v1.20+)
- Triton DataCenter account with NFS volume service
- Triton CloudAPI access

## Installation

### 1. Create a Kubernetes secret with Triton credentials

```bash
kubectl create secret generic triton-creds \
  --namespace=kube-system \
  --from-literal=cloudapi=https://cloudapi.example.triton.zone \
  --from-literal=account-id=your-account-uuid \
  --from-literal=key-id=your-ssh-key-fingerprint \
  --from-file=key.pem=/path/to/your/private/key
```

### 2. Deploy the CSI driver

```bash
# Apply RBAC rules
kubectl apply -f deploy/rbac.yaml

# Deploy controller
kubectl apply -f deploy/controller.yaml

# Deploy node components
kubectl apply -f deploy/node.yaml
```

### 3. Create a StorageClass

```bash
kubectl apply -f deploy/storageclass.yaml
```

## Usage

### Create a PersistentVolumeClaim

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: tritonnfs-pvc
spec:
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 10Gi
  storageClassName: tritonnfs
```

### Use the volume in a Pod

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: test-pod
spec:
  containers:
    - name: test-container
      image: busybox
      command: ["/bin/sh", "-c", "while true; do echo $(date) >> /data/out.txt; sleep 5; done"]
      volumeMounts:
        - name: persistent-storage
          mountPath: /data
  volumes:
    - name: persistent-storage
      persistentVolumeClaim:
        claimName: tritonnfs-pvc
```

## Configuration

The StorageClass supports the following parameters:

- `networks`: Comma-separated list of Triton network IDs to connect the NFS volume to
- `tag-*`: Volume tags (use the `tag-` prefix, e.g., `tag-environment: production`)

### Volume Expansion

To enable volume expansion, ensure the StorageClass has `allowVolumeExpansion: true` set:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: tritonnfs
provisioner: tritonnfs.csi.triton.com
allowVolumeExpansion: true
parameters:
  networks: "1234-5678-9abc-def0"
```

To resize a volume, edit the PVC to request more storage:

```bash
kubectl edit pvc tritonnfs-pvc
```

And update the `spec.resources.requests.storage` field to the new size.

## Building

### Building from Source

The project includes a Makefile to simplify building:

```bash
# Build the binary
make build

# Run tests
make test

# Show current version
make version
```

### Version Management

The driver version is maintained in the `VERSION` file at the root of the repository. To update the version:

1. Edit the `VERSION` file with the new version (e.g., `v0.7.0`)
2. Run `make bump-version` to update all version references in the codebase
3. Run `make docker-build docker-push` to build and push the new image

### Building the Container Image

You can build and tag the container image using:

```bash
# Build with the version from VERSION file
make docker-build

# Build with custom version
make docker-build VERSION=v0.6.0

# Build and push to registry
make docker-push REGISTRY=your-registry IMAGE_TAG=your-tag
```

Or manually:

```bash
docker build --build-arg VERSION=$(cat VERSION) -t tritonnfs-csi:$(cat VERSION) .
```

## Troubleshooting

### Check the driver status

```bash
kubectl get pods -n kube-system -l app=tritonnfs-csi-controller
kubectl get pods -n kube-system -l app=tritonnfs-csi-node
```

### View logs

```bash
# Controller logs
kubectl logs -n kube-system -l app=tritonnfs-csi-controller -c tritonnfs-csi-plugin

# Node logs (use the appropriate node name)
kubectl logs -n kube-system -l app=tritonnfs-csi-node -c tritonnfs-csi-plugin
```

## Limitations

- Snapshots and clones are not yet supported
- Authentication:
  - HTTP signature authentication is implemented and working with SSH keys
  - Both SSH agent authentication and direct key file authentication are supported

## License

Apache License 2.0