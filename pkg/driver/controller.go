package driver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// VolumeTypeNFS is the volume type for NFS volumes
	VolumeTypeNFS = "nfs"

	// Topology Keys
	TopologyKeyZone = "topology.tritonnfs.csi.triton.com/zone"

	// Default size in bytes (10GB)
	DefaultVolumeSizeBytes int64 = 10 * 1024 * 1024 * 1024
)

var (
	// ControllerCapabilities defines the capabilities of the controller service
	ControllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
		csi.ControllerServiceCapability_RPC_GET_CAPACITY,
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
		csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	}
)

// ControllerGetCapabilities returns the capabilities of the controller service
func (d *TritonNFSDriver) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	var caps []*csi.ControllerServiceCapability
	for _, cap := range ControllerCapabilities {
		caps = append(caps, &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		})
	}
	return &csi.ControllerGetCapabilitiesResponse{Capabilities: caps}, nil
}

// CreateVolume creates a new volume
func (d *TritonNFSDriver) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	// Validate arguments
	if req.GetName() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume name must be provided")
	}

	// Get volume parameters
	volumeCapabilities := req.GetVolumeCapabilities()
	if len(volumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities must be provided")
	}

	// Check if volume capabilities are supported
	for _, capability := range volumeCapabilities {
		if capability.GetMount() == nil {
			return nil, status.Error(codes.InvalidArgument, "Only mount volumes are supported")
		}
	}

	// Get volume size
	size := DefaultVolumeSizeBytes
	if req.GetCapacityRange() != nil && req.GetCapacityRange().GetRequiredBytes() > 0 {
		size = req.GetCapacityRange().GetRequiredBytes()
	}

	// Check if volume already exists
	volumes, err := d.tritonClient.ListVolumes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list volumes: %v", err)
	}

	// Check if a volume with the same name already exists
	for _, vol := range volumes {
		if vol.Name == req.GetName() {
			// Check if the existing volume satisfies the request
			if vol.Size >= size {
				// Return the existing volume
				return &csi.CreateVolumeResponse{
					Volume: &csi.Volume{
						VolumeId:      vol.ID,
						CapacityBytes: vol.Size,
						VolumeContext: map[string]string{
							"server":     getVolumeServer(vol),
							"share":      vol.MountPoint,
							"type":       VolumeTypeNFS,
							"volumeName": vol.Name,
						},
					},
				}, nil
			}
			// Existing volume doesn't satisfy the request
			return nil, status.Errorf(codes.AlreadyExists, "Volume with name %s already exists but with different size", req.GetName())
		}
	}

	// Create volume request
	volumeRequest := &NFSVolumeRequest{
		Name: req.GetName(),
		Size: size,
		Type: VolumeTypeNFS,
	}

	// Get parameters from volume context
	params := req.GetParameters()
	if params != nil {
		// Handle networks if provided
		if networksStr, ok := params["networks"]; ok && networksStr != "" {
			volumeRequest.Networks = strings.Split(networksStr, ",")
		}

		// Handle tags if provided
		volumeRequest.Tags = make(map[string]string)
		volumeRequest.Tags["created-by"] = "tritonnfs-csi-driver"
		for k, v := range params {
			if strings.HasPrefix(k, "tag-") {
				tagKey := strings.TrimPrefix(k, "tag-")
				volumeRequest.Tags[tagKey] = v
			}
		}
	}

	// Create the volume
	volume, err := d.tritonClient.CreateVolume(ctx, volumeRequest)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create volume: %v", err)
	}

	// Wait for volume to be ready
	volume, err = waitForVolumeReady(ctx, d.tritonClient, volume.ID)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed waiting for volume to become ready: %v", err)
	}

	// Return the created volume
	return &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volume.ID,
			CapacityBytes: volume.Size,
			VolumeContext: map[string]string{
				"server":     getVolumeServer(volume),
				"share":      volume.MountPoint,
				"type":       VolumeTypeNFS,
				"volumeName": volume.Name,
			},
		},
	}, nil
}

// DeleteVolume deletes a volume
func (d *TritonNFSDriver) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	// Delete the volume
	err := d.tritonClient.DeleteVolume(ctx, req.GetVolumeId())
	if err != nil {
		// Volume not found is not an error
		if strings.Contains(err.Error(), "404") {
			logrus.Warnf("Volume %s not found, assuming it's already deleted", req.GetVolumeId())
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, status.Errorf(codes.Internal, "Failed to delete volume: %v", err)
	}

	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume is called when a volume is attached to a node
func (d *TritonNFSDriver) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	// NFS volumes don't require controller publish
	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume is called when a volume is detached from a node
func (d *TritonNFSDriver) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	// NFS volumes don't require controller unpublish
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities validates if a volume has the given capabilities
func (d *TritonNFSDriver) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	if len(req.GetVolumeCapabilities()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume capabilities must be provided")
	}

	// Check if volume exists
	_, err := d.tritonClient.GetVolume(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume with ID %s not found: %v", req.GetVolumeId(), err)
	}

	// Check if volume capabilities are supported
	for _, capability := range req.GetVolumeCapabilities() {
		if capability.GetMount() == nil {
			return &csi.ValidateVolumeCapabilitiesResponse{
				Confirmed: nil,
				Message:   "Only mount volumes are supported",
			}, nil
		}
	}

	// All capabilities are supported
	return &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: req.GetVolumeCapabilities(),
		},
	}, nil
}

// ListVolumes lists all volumes
func (d *TritonNFSDriver) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	// Pagination support isn't implemented yet
	if req.GetStartingToken() != "" {
		return nil, status.Error(codes.Unimplemented, "Pagination is not implemented")
	}

	// List volumes
	volumes, err := d.tritonClient.ListVolumes(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list volumes: %v", err)
	}

	// Build response
	var entries []*csi.ListVolumesResponse_Entry
	for _, vol := range volumes {
		entries = append(entries, &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{
				VolumeId:      vol.ID,
				CapacityBytes: vol.Size,
				VolumeContext: map[string]string{
					"server":     getVolumeServer(vol),
					"share":      vol.MountPoint,
					"type":       VolumeTypeNFS,
					"volumeName": vol.Name,
				},
			},
		})
	}

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// GetCapacity returns the available capacity
func (d *TritonNFSDriver) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	// This is not implemented because CloudAPI does not provide capacity information
	return &csi.GetCapacityResponse{
		AvailableCapacity: 0,
	}, nil
}

// ControllerExpandVolume expands a volume
func (d *TritonNFSDriver) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}
	
	if req.GetCapacityRange() == nil {
		return nil, status.Error(codes.InvalidArgument, "Capacity range must be provided")
	}
	
	requiredBytes := req.GetCapacityRange().GetRequiredBytes()
	if requiredBytes <= 0 {
		return nil, status.Error(codes.InvalidArgument, "Required bytes must be greater than 0")
	}
	
	// Get the current volume
	volume, err := d.tritonClient.GetVolume(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume with ID %s not found: %v", req.GetVolumeId(), err)
	}
	
	// Check if resizing is needed
	if volume.Size >= requiredBytes {
		// Volume is already larger than the requested size, no resizing needed
		return &csi.ControllerExpandVolumeResponse{
			CapacityBytes:         volume.Size,
			NodeExpansionRequired: false, // NFS volumes do not require node expansion
		}, nil
	}
	
	// Expand the volume
	expandedVolume, err := d.tritonClient.ExpandVolume(ctx, req.GetVolumeId(), requiredBytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to expand volume: %v", err)
	}
	
	// Return the new size
	return &csi.ControllerExpandVolumeResponse{
		CapacityBytes:         expandedVolume.Size,
		NodeExpansionRequired: false, // NFS volumes do not require node expansion
	}, nil
}

// CreateSnapshot creates a snapshot
func (d *TritonNFSDriver) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	// Not implemented yet
	return nil, status.Error(codes.Unimplemented, "CreateSnapshot is not implemented")
}

// DeleteSnapshot deletes a snapshot
func (d *TritonNFSDriver) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	// Not implemented yet
	return nil, status.Error(codes.Unimplemented, "DeleteSnapshot is not implemented")
}

// ListSnapshots lists all snapshots
func (d *TritonNFSDriver) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	// Not implemented yet
	return nil, status.Error(codes.Unimplemented, "ListSnapshots is not implemented")
}

// ControllerGetVolume gets volume info
func (d *TritonNFSDriver) ControllerGetVolume(ctx context.Context, req *csi.ControllerGetVolumeRequest) (*csi.ControllerGetVolumeResponse, error) {
	// Validate arguments
	if req.GetVolumeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	// Get volume
	volume, err := d.tritonClient.GetVolume(ctx, req.GetVolumeId())
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Volume with ID %s not found: %v", req.GetVolumeId(), err)
	}

	// Build response
	return &csi.ControllerGetVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volume.ID,
			CapacityBytes: volume.Size,
			VolumeContext: map[string]string{
				"server":     getVolumeServer(volume),
				"share":      volume.MountPoint,
				"type":       VolumeTypeNFS,
				"volumeName": volume.Name,
			},
		},
	}, nil
}

// ControllerModifyVolume modifies a volume
func (d *TritonNFSDriver) ControllerModifyVolume(ctx context.Context, req *csi.ControllerModifyVolumeRequest) (*csi.ControllerModifyVolumeResponse, error) {
	// Not implemented yet
	return nil, status.Error(codes.Unimplemented, "ControllerModifyVolume is not implemented")
}

// Helper function to get the NFS server IP from the volume
func getVolumeServer(volume *NFSVolume) string {
	if len(volume.Networks) > 0 {
		return volume.Networks[0].IP
	}
	return ""
}

// waitForVolumeReady waits for a volume to become ready
func waitForVolumeReady(ctx context.Context, client *TritonClient, volumeID string) (*NFSVolume, error) {
	// Maximum number of retries
	maxRetries := 30
	// Retry interval in seconds
	retryInterval := 5

	for i := 0; i < maxRetries; i++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled while waiting for volume to be ready")
		default:
		}

		// Get volume status
		volume, err := client.GetVolume(ctx, volumeID)
		if err != nil {
			logrus.Errorf("Failed to get volume status: %v", err)
			return nil, err
		}

		// Check if volume is ready
		if volume.State == "ready" {
			return volume, nil
		}

		// If volume is in error state, return error
		if volume.State == "error" {
			return nil, fmt.Errorf("volume is in error state")
		}

		// Log current state
		logrus.Infof("Volume %s is in state %s, waiting %d seconds...", volumeID, volume.State, retryInterval)

		// Wait before retrying
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}

	return nil, fmt.Errorf("timed out waiting for volume to be ready")
}