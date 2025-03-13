# Volume Expansion with TritonNFS CSI Driver

This document explains how to use the volume expansion feature of the TritonNFS CSI Driver to resize your volumes.

## Prerequisites

- Kubernetes 1.16+ (required for volume expansion)
- TritonNFS CSI Driver properly installed
- StorageClass with `allowVolumeExpansion: true` (the default storageclass.yaml already includes this)

## Steps to Expand a Volume

### 1. Create a PVC

First, create a PVC using the TritonNFS StorageClass. Here's a sample PVC:

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

Apply it to your cluster:

```bash
kubectl apply -f pvc.yaml
```

### 2. Expand the Volume

To expand the volume, edit the PVC to request more storage:

```bash
kubectl edit pvc tritonnfs-pvc
```

Change the `spec.resources.requests.storage` value to a larger size, for example, from 10Gi to 20Gi:

```yaml
spec:
  resources:
    requests:
      storage: 20Gi  # Changed from 10Gi
```

### 3. Verify the Expansion

After editing the PVC, it will automatically trigger the volume expansion. You can check the status with:

```bash
kubectl get pvc tritonnfs-pvc
```

The status should eventually show the increased capacity. This may take a few moments to complete as the Triton API processes the resize request.

## Example with Pod Usage

Create a pod that uses the PVC:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: volume-test
spec:
  containers:
  - name: volume-test
    image: nginx
    volumeMounts:
    - name: nfs-volume
      mountPath: /data
    command: ["sh", "-c", "while true; do df -h /data; sleep 60; done"]
  volumes:
  - name: nfs-volume
    persistentVolumeClaim:
      claimName: tritonnfs-pvc
```

Once the volume expansion is complete, the pod will automatically see the increased capacity without needing to restart.

## Notes

- Volume expansion in TritonNFS is online, meaning pods using the volume don't need to be restarted
- You can only increase the size of a volume, not decrease it
- The size is always rounded up to the nearest gigabyte (GB)

## Troubleshooting

If the volume expansion is not working as expected, check the controller logs:

```bash
kubectl logs -n kube-system -l app=tritonnfs-csi-controller -c tritonnfs-csi-plugin
```

Look for entries related to `ControllerExpandVolume` which will contain information about the expansion process.