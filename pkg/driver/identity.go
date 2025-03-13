package driver

import (
	"context"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// GetPluginInfo returns metadata about the CSI plugin
func (d *TritonNFSDriver) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	if d.nodeID == "" {
		return nil, status.Error(codes.Unavailable, "Node ID not configured")
	}

	resp := &csi.GetPluginInfoResponse{
		Name:          "tritonnfs.csi.triton.com",
		VendorVersion: DriverVersion,
	}

	return resp, nil
}

// GetPluginCapabilities returns the capabilities of the CSI plugin
func (d *TritonNFSDriver) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_ONLINE,
					},
				},
			},
		},
	}

	return resp, nil
}

// Probe returns the health and readiness of the CSI plugin
func (d *TritonNFSDriver) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	return &csi.ProbeResponse{}, nil
}