package driver

import (
	"context"
	"fmt"
	"io/ioutil"
	"strings"
	"time"
	"encoding/pem"

	triton "github.com/joyent/triton-go/v2"
	"github.com/joyent/triton-go/v2/authentication"
	"github.com/joyent/triton-go/v2/compute"
	"github.com/sirupsen/logrus"
)

// TritonClient is a client for the Triton CloudAPI
type TritonClient struct {
	computeClient *compute.ComputeClient
	endpoint      string
	accountID     string
	keyID         string
	keyPath       string
}

// NewTritonClient creates a new TritonClient with the given options
func NewTritonClient(endpoint, accountID, keyID, keyPath string) (*TritonClient, error) {
	logrus.Infof("Creating Triton client with endpoint: %s, accountID: %s, keyID: %s, keyPath: %s", endpoint, accountID, keyID, keyPath)
	
	// Read the SSH private key file
	privateKeyData, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %v", err)
	}

	// Log the first few characters of the private key for debugging
	if len(privateKeyData) > 50 {
		logrus.Infof("Private key starts with: %s", string(privateKeyData[:50]))
	}
	
	// Parse the private key
	block, _ := pem.Decode(privateKeyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block containing private key")
	}

	// Check if it's an encrypted key
	if block.Headers["Proc-Type"] == "4,ENCRYPTED" {
		return nil, fmt.Errorf("encrypted private keys are not supported, please decrypt the key first")
	}
	
	// Try to create a private key signer
	input := authentication.PrivateKeySignerInput{
		KeyID:              keyID,
		PrivateKeyMaterial: privateKeyData,
		AccountName:        accountID,
	}
	
	signer, err := authentication.NewPrivateKeySigner(input)
	if err != nil {
		logrus.Errorf("Error creating private key signer: %v", err)
		
		// Try a different way of parsing the key - for now just return the error
		return nil, fmt.Errorf("failed to create private key signer: %v", err)
	}
	
	// Create a Triton client config
	config := &triton.ClientConfig{
		TritonURL:   endpoint,
		AccountName: accountID,
		Signers:     []authentication.Signer{signer},
	}
	
	// Create the compute client
	computeClient, err := compute.NewClient(config)
	if err != nil {
		logrus.Errorf("Failed to create compute client: %v", err)
		return nil, fmt.Errorf("failed to create compute client: %v", err)
	}
	
	// Verify connection with a simple API call
	logrus.Infof("Testing connection to Triton API")
	volumes, err := computeClient.Volumes().List(context.Background(), &compute.ListVolumesInput{})
	if err != nil {
		logrus.Errorf("Failed to list volumes using triton-go client: %v", err)
		return nil, fmt.Errorf("failed to connect to Triton API: %v", err)
	}
	
	logrus.Infof("Successfully connected to Triton API, found %d volumes", len(volumes))
	
	return &TritonClient{
		computeClient: computeClient,
		endpoint:      endpoint,
		accountID:     accountID,
		keyID:         keyID,
		keyPath:       keyPath,
	}, nil
}

// NFSVolumeRequest represents a request to create an NFS volume
type NFSVolumeRequest struct {
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	Networks   []string          `json:"networks,omitempty"`
	Tags       map[string]string `json:"tags,omitempty"`
	Type       string            `json:"type"`
	FifoSize   int64             `json:"fifo_size,omitempty"`
	MountPoint string            `json:"mountpoint,omitempty"`
}

// NFSVolume represents an NFS volume in Triton
type NFSVolume struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	State          string            `json:"state"`
	Type           string            `json:"type"`
	Networks       []Network         `json:"networks"`
	Size           int64             `json:"size"`
	MountPoint     string            `json:"mountpoint"`
	FileSystemPath string            `json:"filesystem_path"` // Added filesystem_path field
	Created        time.Time         `json:"created"`
	Tags           map[string]string `json:"tags"`
	Server         string            `json:"server"`         // Server IP for the NFS volume
}

// Network represents a network in Triton
type Network struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

// CreateVolume creates a new NFS volume
func (c *TritonClient) CreateVolume(ctx context.Context, req *NFSVolumeRequest) (*NFSVolume, error) {
	logrus.Infof("Creating volume with name: %s, size: %d", req.Name, req.Size)
	
	// Use the triton-go compute client
	if c.computeClient == nil {
		return nil, fmt.Errorf("compute client not initialized")
	}
	
	// Convert size from bytes to megabytes (Triton expects size in MB)
	// The error message shows sizes like 10240, 20480, which are in MB
	sizeMB := req.Size / (1024 * 1024)
	if req.Size%(1024*1024) > 0 {
		sizeMB++ // Round up to the next MB
	}
	
	// Create volume input
	createInput := &compute.CreateVolumeInput{
		Name: req.Name,
		Type: req.Type,
		Size: int64(sizeMB),
		Tags: req.Tags,
	}
	
	if len(req.Networks) > 0 {
		createInput.Networks = req.Networks
	}
	
	// Create the volume
	volume, err := c.computeClient.Volumes().Create(ctx, createInput)
	if err != nil {
		logrus.Errorf("Failed to create volume using triton-go client: %v", err)
		return nil, err
	}
	
	// Wait for the volume to be ready
	volumeID := volume.ID
	
	// Poll for volume state until it's ready or fails
	readyVolume, err := c.waitForVolumeReady(ctx, volumeID)
	if err != nil {
		logrus.Errorf("Volume creation initiated but failed to reach ready state: %v", err)
		return nil, err
	}
	
	// Check if FileSystemPath exists
	if readyVolume.FileSystemPath == "" {
		logrus.Warnf("Volume %s is ready but has no FileSystemPath", volumeID)
	} else {
		logrus.Infof("Volume %s created with FileSystemPath: %s", volumeID, readyVolume.FileSystemPath)
	}
	
	// Convert to our internal NFSVolume type
	nfsVolume := &NFSVolume{
		ID:             readyVolume.ID,
		Name:           readyVolume.Name,
		State:          readyVolume.State,
		Type:           readyVolume.Type,
		Size:           int64(readyVolume.Size) * 1024 * 1024, // Convert MB to bytes
		MountPoint:     readyVolume.FileSystemPath,
		FileSystemPath: readyVolume.FileSystemPath, // This contains the full NFS path including IP
		Created:        time.Now(), // No Created field in compute.Volume
		Tags:           readyVolume.Tags,
		Networks:       []Network{}, // Initialize with empty slice
	}
	
	// Add networks from the volume
	// In the triton-go library, Networks might be a []string of network IDs
	for _, netID := range readyVolume.Networks {
		nfsVolume.Networks = append(nfsVolume.Networks, Network{
			ID:   netID,
			Name: "network",
			IP:   "",
		})
	}
	
	return nfsVolume, nil
}

// GetVolume gets a volume by ID
func (c *TritonClient) GetVolume(ctx context.Context, id string) (*NFSVolume, error) {
	logrus.Infof("Getting volume with ID: %s", id)
	
	// Use the triton-go compute client
	if c.computeClient == nil {
		return nil, fmt.Errorf("compute client not initialized")
	}
	
	// Kubernetes appends "-id" to volume IDs in certain contexts
	// Strip it if present to match the format expected by Triton API
	if strings.HasSuffix(id, "-id") {
		originalID := id
		id = strings.TrimSuffix(id, "-id")
		logrus.Infof("Trimmed '-id' suffix from volume ID: %s → %s", originalID, id)
	}
	
	// Get the volume from Triton
	volume, err := c.computeClient.Volumes().Get(ctx, &compute.GetVolumeInput{
		ID: id,
	})
	
	if err != nil {
		logrus.Errorf("Failed to get volume using triton-go client: %v", err)
		return nil, err
	}
	
	// Check if FileSystemPath exists and dump volume details for debugging
	logrus.Infof("Volume details for %s: ID=%s, Name=%s, State=%s, Type=%s", 
		id, volume.ID, volume.Name, volume.State, volume.Type)
	logrus.Infof("Volume networks for %s: %v", id, volume.Networks)
	
	if volume.FileSystemPath == "" {
		logrus.Warnf("Volume %s has no FileSystemPath", id)
	} else {
		logrus.Infof("Volume %s has FileSystemPath: %s", id, volume.FileSystemPath)
	}
	
	// Convert to our internal NFSVolume type
	nfsVolume := &NFSVolume{
		ID:             volume.ID,
		Name:           volume.Name,
		State:          volume.State,
		Type:           volume.Type,
		Size:           int64(volume.Size) * 1024 * 1024, // Convert MB to bytes
		MountPoint:     volume.FileSystemPath,
		FileSystemPath: volume.FileSystemPath, // This contains the full NFS path including IP
		Created:        time.Now(), // No Created field in compute.Volume
		Tags:           volume.Tags,
		Networks:       []Network{}, // Initialize with empty slice
	}
	
	// Add networks from the volume
	// In the triton-go library, Networks is a []string of network IDs
	for _, netID := range volume.Networks {
		nfsVolume.Networks = append(nfsVolume.Networks, Network{
			ID:   netID,
			Name: "network",
			IP:   "",
		})
	}
	
	return nfsVolume, nil
}

// DeleteVolume deletes a volume by ID
func (c *TritonClient) DeleteVolume(ctx context.Context, id string) error {
	logrus.Infof("Deleting volume with ID: %s", id)
	
	// Use the triton-go compute client
	if c.computeClient == nil {
		return fmt.Errorf("compute client not initialized")
	}
	
	// Kubernetes appends "-id" to volume IDs in certain contexts
	// Strip it if present to match the format expected by Triton API
	if strings.HasSuffix(id, "-id") {
		originalID := id
		id = strings.TrimSuffix(id, "-id")
		logrus.Infof("Trimmed '-id' suffix from volume ID: %s → %s", originalID, id)
	}
	
	// Delete the volume using the Triton API
	err := c.computeClient.Volumes().Delete(ctx, &compute.DeleteVolumeInput{
		ID: id,
	})
	
	if err != nil {
		logrus.Errorf("Failed to delete volume using triton-go client: %v", err)
		return err
	}
	
	// Successful deletion
	logrus.Infof("Volume %s deleted successfully", id)
	return nil
}

// ExpandVolume expands an existing volume to a new size
func (c *TritonClient) ExpandVolume(ctx context.Context, id string, newSize int64) (*NFSVolume, error) {
	logrus.Infof("Expanding volume with ID: %s to new size: %d bytes", id, newSize)
	
	// Use the triton-go compute client
	if c.computeClient == nil {
		return nil, fmt.Errorf("compute client not initialized")
	}
	
	// Kubernetes appends "-id" to volume IDs in certain contexts
	// Strip it if present to match the format expected by Triton API
	if strings.HasSuffix(id, "-id") {
		originalID := id
		id = strings.TrimSuffix(id, "-id")
		logrus.Infof("Trimmed '-id' suffix from volume ID: %s → %s", originalID, id)
	}
	
	// First get the current volume
	currentVolume, err := c.computeClient.Volumes().Get(ctx, &compute.GetVolumeInput{
		ID: id,
	})
	
	if err != nil {
		logrus.Errorf("Failed to get volume using triton-go client: %v", err)
		return nil, err
	}
	
	// Check if resizing is needed
	currentSizeBytes := int64(currentVolume.Size) * 1024 * 1024 // Convert MB to bytes
	if currentSizeBytes >= newSize {
		logrus.Infof("Volume %s already has sufficient size (%d bytes), no resize needed", 
			id, currentSizeBytes)
			
		// Return the current volume since it's already large enough
		nfsVolume := &NFSVolume{
			ID:             currentVolume.ID,
			Name:           currentVolume.Name,
			State:          currentVolume.State,
			Type:           currentVolume.Type,
			Size:           currentSizeBytes,
			MountPoint:     currentVolume.FileSystemPath,
			FileSystemPath: currentVolume.FileSystemPath, // Use the full filesystem path from Triton API
			Created:        time.Now(), // No Created field in compute.Volume
			Tags:           currentVolume.Tags,
			Networks:       []Network{}, // Initialize with empty slice
		}
		
		// Add networks from the volume
		for _, netID := range currentVolume.Networks {
			nfsVolume.Networks = append(nfsVolume.Networks, Network{
				ID:   netID,
				Name: "network",
				IP:   "",
			})
		}
		
		return nfsVolume, nil
	}
	
	// Volume expansion is not implemented yet
	logrus.Warnf("Volume expansion not implemented yet. Cannot expand volume %s to size %d bytes", id, newSize)
	return nil, fmt.Errorf("volume expansion not implemented yet")
}

// ListVolumes lists all volumes
func (c *TritonClient) ListVolumes(ctx context.Context) ([]*NFSVolume, error) {
	logrus.Infof("Listing volumes")
	
	// Use the triton-go compute client
	if c.computeClient == nil {
		return nil, fmt.Errorf("compute client not initialized")
	}
	
	// List all volumes
	tritonVolumes, err := c.computeClient.Volumes().List(ctx, &compute.ListVolumesInput{})
	
	if err != nil {
		logrus.Errorf("Failed to list volumes using triton-go client: %v", err)
		return nil, err
	}
	
	logrus.Infof("Found %d volumes from Triton API", len(tritonVolumes))
	
	// Convert to our internal NFSVolume type
	var volumes []*NFSVolume
	for _, vol := range tritonVolumes {
		// Skip non-tritonnfs volumes
		if vol.Type != "tritonnfs" {
			logrus.Warnf("Skipping volume %s with type %s (only tritonnfs is supported)", vol.ID, vol.Type)
			continue
		}
		
		// Check if FileSystemPath exists
		if vol.FileSystemPath == "" {
			logrus.Warnf("Volume %s has no FileSystemPath", vol.ID)
		} else {
			logrus.Infof("Volume %s has FileSystemPath: %s", vol.ID, vol.FileSystemPath)
		}
		
		nfsVolume := &NFSVolume{
			ID:             vol.ID,
			Name:           vol.Name,
			State:          vol.State,
			Type:           vol.Type,
			Size:           int64(vol.Size) * 1024 * 1024, // Convert MB to bytes
			MountPoint:     vol.FileSystemPath,
			FileSystemPath: vol.FileSystemPath, // This contains the full NFS path including IP
			Created:        time.Now(), // No Created field in compute.Volume
			Tags:           vol.Tags,
			Networks:       []Network{}, // Initialize with empty slice
		}
		
		// Add networks from the volume
		for _, netID := range vol.Networks {
			nfsVolume.Networks = append(nfsVolume.Networks, Network{
				ID:   netID,
				Name: "network",
				IP:   "",
			})
		}
		
		volumes = append(volumes, nfsVolume)
	}
	
	return volumes, nil
}

// waitForVolumeReady polls the volume until it reaches the "ready" state
func (c *TritonClient) waitForVolumeReady(ctx context.Context, volumeID string) (*compute.Volume, error) {
	// Define polling parameters
	maxAttempts := 30
	pollInterval := 10 * time.Second
	
	// Poll for volume status
	for attempt := 0; attempt < maxAttempts; attempt++ {
		// Check if context is cancelled
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
		
		// Get current volume status
		volume, err := c.computeClient.Volumes().Get(ctx, &compute.GetVolumeInput{
			ID: volumeID,
		})
		
		if err != nil {
			return nil, fmt.Errorf("failed to get volume status: %v", err)
		}
		
		// Check if volume is ready
		if volume.State == "ready" {
			// Add more debugging for filesystem_path
			logrus.Infof("Volume %s is ready with FileSystemPath: %s", volumeID, volume.FileSystemPath)
			if volume.FileSystemPath == "" {
				logrus.Warnf("Volume %s is ready but has no FileSystemPath", volumeID)
			}
			return volume, nil
		}
		
		// Check if volume is in a failed state
		if volume.State == "failed" {
			return nil, fmt.Errorf("volume reached failed state")
		}
		
		// Log the current state and continue polling
		logrus.Infof("Volume %s is in %s state, waiting %v before checking again (attempt %d/%d)", 
			volumeID, volume.State, pollInterval, attempt+1, maxAttempts)
		
		// Wait before next attempt
		time.Sleep(pollInterval)
	}
	
	return nil, fmt.Errorf("timed out waiting for volume to be ready after %d attempts", maxAttempts)
}