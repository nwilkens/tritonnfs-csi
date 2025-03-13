package driver

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// NFSDefaultPort is the default port for NFS
	NFSDefaultPort = "2049"
)

// NodeStageVolume stages a volume on the node
func (d *TritonNFSDriver) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	// NFS volumes don't require staging
	return &csi.NodeStageVolumeResponse{}, nil
}

// NodeUnstageVolume unstages a volume from the node
func (d *TritonNFSDriver) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	// NFS volumes don't require unstaging
	return &csi.NodeUnstageVolumeResponse{}, nil
}

// NodePublishVolume mounts a volume to the target path
func (d *TritonNFSDriver) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path must be provided")
	}
	if req.GetVolumeCapability() == nil {
		return nil, status.Error(codes.InvalidArgument, "Volume capability must be provided")
	}

	// Check if the target path exists
	targetPath := req.GetTargetPath()
	notMount, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create the directory if it doesn't exist
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Errorf(codes.Internal, "Failed to create directory %s: %v", targetPath, err)
			}
			notMount = true
		} else {
			return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
		}
	}

	// If already mounted, return
	if !notMount {
		return &csi.NodePublishVolumeResponse{}, nil
	}

	// Get the volume context
	volumeContext := req.GetVolumeContext()
	server, ok := volumeContext["server"]
	if !ok || server == "" {
		return nil, status.Error(codes.InvalidArgument, "server must be provided in volume context")
	}

	share, ok := volumeContext["share"]
	if !ok || share == "" {
		return nil, status.Error(codes.InvalidArgument, "share must be provided in volume context")
	}

	// Add default port if not present
	if !strings.Contains(server, ":") {
		server = server + ":" + NFSDefaultPort
	}

	// Get mount options from volume capability
	mountOptions := []string{"nolock"}
	if mount := req.GetVolumeCapability().GetMount(); mount != nil {
		for _, opt := range mount.GetMountFlags() {
			mountOptions = append(mountOptions, opt)
		}
	}

	// Add readonly option if specified
	if req.GetReadonly() {
		mountOptions = append(mountOptions, "ro")
	}

	// Format the source
	source := fmt.Sprintf("%s:%s", server, share)

	// Mount the volume
	logrus.Infof("Mounting NFS volume %s from %s to %s with options %v", req.GetVolumeId(), source, targetPath, mountOptions)
	if err := d.mounter.Mount(source, targetPath, "nfs", mountOptions); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to mount volume %s to %s: %v", source, targetPath, err)
	}

	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume unmounts a volume from the target path
func (d *TritonNFSDriver) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	if req.GetTargetPath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Target path must be provided")
	}

	// Check if the target path exists
	targetPath := req.GetTargetPath()
	notMount, err := d.mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Already unmounted
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Failed to check mount point: %v", err)
	}

	// Unmount if mounted
	if !notMount {
		logrus.Infof("Unmounting volume %s from %s", req.GetVolumeId(), targetPath)
		if err := d.mounter.Unmount(targetPath); err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to unmount volume: %v", err)
		}
	}

	// Remove the directory
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Failed to remove directory %s: %v", targetPath, err)
	}

	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeGetCapabilities returns the capabilities of the node service
func (d *TritonNFSDriver) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
					},
				},
			},
		},
	}, nil
}

// NodeGetInfo returns info about the node
func (d *TritonNFSDriver) NodeGetInfo(ctx context.Context, req *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	return &csi.NodeGetInfoResponse{
		NodeId: d.nodeID,
	}, nil
}

// NodeGetVolumeStats returns stats about the volume
func (d *TritonNFSDriver) NodeGetVolumeStats(ctx context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	if req.GetVolumePath() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume path must be provided")
	}

	// Check if volume path exists
	volumePath := req.GetVolumePath()
	_, err := os.Stat(volumePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, status.Errorf(codes.NotFound, "Volume path %s does not exist", volumePath)
		}
		return nil, status.Errorf(codes.Internal, "Failed to stat volume path: %v", err)
	}

	// For NFS, we can't get accurate stats, so we just return that the volume exists
	return &csi.NodeGetVolumeStatsResponse{}, nil
}

// NodeExpandVolume expands a volume on the node
func (d *TritonNFSDriver) NodeExpandVolume(ctx context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	// NFS volumes don't require node expansion
	return &csi.NodeExpandVolumeResponse{}, nil
}