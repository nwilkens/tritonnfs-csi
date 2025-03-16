package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/joyent/tritonnfs-csi/pkg/driver"
	"github.com/sirupsen/logrus"
)

func main() {
	// Get authentication details from environment variables
	cloudAPI := os.Getenv("TRITON_CLOUD_API")
	accountID := os.Getenv("TRITON_ACCOUNT_ID")
	keyID := os.Getenv("TRITON_KEY_ID")
	keyPath := os.Getenv("TRITON_PRIVATE_KEY_FILE")

	// Validate required environment variables
	if cloudAPI == "" || accountID == "" || keyID == "" || keyPath == "" {
		fmt.Println("ERROR: Required environment variables not set.")
		fmt.Println("Please set the following environment variables:")
		fmt.Println("  TRITON_CLOUD_API - e.g. https://us-central-1.api.mnx.io")
		fmt.Println("  TRITON_ACCOUNT_ID - Your Triton account ID/name")
		fmt.Println("  TRITON_KEY_ID - Your SSH key fingerprint")
		fmt.Println("  TRITON_PRIVATE_KEY_FILE - Path to your SSH private key file")
		os.Exit(1)
	}

	// Initialize logger
	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp: true,
	})
	logrus.SetOutput(os.Stdout)
	logrus.SetLevel(logrus.InfoLevel)

	fmt.Println("=== Triton Volume Operations Test ===")
	fmt.Printf("Cloud API: %s\n", cloudAPI)
	fmt.Printf("Account ID: %s\n", accountID)
	fmt.Printf("Key ID: %s\n", keyID)
	fmt.Printf("Key Path: %s\n", keyPath)

	// Step 1: Create Triton client (this also validates authentication)
	fmt.Println("\n1. Creating Triton client (auth test)")
	client, err := driver.NewTritonClient(cloudAPI, accountID, keyID, keyPath)
	if err != nil {
		fmt.Printf("ERROR: Failed to create Triton client: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Authentication successful!")

	// Generate a unique test volume name with PVC prefix like Kubernetes
	testVolumeName := fmt.Sprintf("pvc-test-%d", time.Now().Unix())
	fmt.Printf("\nUsing test volume name: %s\n", testVolumeName)

	// Step 2: List volumes (before creation)
	fmt.Println("\n2. Listing volumes (before creation)")
	volumes, err := client.ListVolumes(context.Background())
	if err != nil {
		fmt.Printf("ERROR: Failed to list volumes: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Found %d volumes\n", len(volumes))
	for i, vol := range volumes {
		fmt.Printf("  %d. Volume ID: %s, Name: %s, Type: %s, State: %s\n", 
			i+1, vol.ID, vol.Name, vol.Type, vol.State)
	}

	// Step 3: Create a test volume
	fmt.Printf("\n3. Creating test volume: %s\n", testVolumeName)
	volReq := &driver.NFSVolumeRequest{
		Name: testVolumeName,
		Size: 10 * 1024 * 1024 * 1024, // 10 GB in bytes (will be converted to 10240 MB)
		Type: "tritonnfs",
		Tags: map[string]string{
			"created-by": "tritonnfs-csi-test",
		},
	}
	
	newVolume, err := client.CreateVolume(context.Background(), volReq)
	if err != nil {
		fmt.Printf("ERROR: Failed to create volume: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✓ Volume created successfully!\n")
	fmt.Printf("  Volume ID: %s\n", newVolume.ID)
	fmt.Printf("  Volume Name: %s\n", newVolume.Name)
	fmt.Printf("  Volume Type: %s\n", newVolume.Type)
	fmt.Printf("  Volume State: %s\n", newVolume.State)
	fmt.Printf("  Volume Size: %d bytes\n", newVolume.Size)
	fmt.Printf("  Mount Point: %s\n", newVolume.MountPoint)
	fmt.Printf("  FileSystem Path: %s\n", newVolume.FileSystemPath)
	
	// Parse and display the server IP and mount point details
	// FileSystemPath format is typically: server-ip:/mount/path
	if newVolume.FileSystemPath != "" {
		parts := strings.Split(newVolume.FileSystemPath, ":")
		if len(parts) >= 2 {
			serverIP := parts[0]
			mountPath := strings.Join(parts[1:], ":")
			fmt.Printf("\n  Mount Details:\n")
			fmt.Printf("  • Server IP: %s\n", serverIP)
			fmt.Printf("  • Mount Path: %s\n", mountPath)
			fmt.Printf("  • NFS Mount Command: mount -t nfs %s:%s /mnt/target\n", serverIP, mountPath)
		}
	}

	// Step 4: Get the created volume
	fmt.Printf("\n4. Getting volume by ID: %s\n", newVolume.ID)
	getVolume, err := client.GetVolume(context.Background(), newVolume.ID)
	if err != nil {
		fmt.Printf("ERROR: Failed to get volume: %v\n", err)
		os.Exit(1)
	}
	
	fmt.Printf("✓ Retrieved volume successfully!\n")
	fmt.Printf("  Volume ID: %s\n", getVolume.ID)
	fmt.Printf("  Volume Name: %s\n", getVolume.Name)
	fmt.Printf("  Volume Type: %s\n", getVolume.Type)
	fmt.Printf("  Volume State: %s\n", getVolume.State)
	fmt.Printf("  Volume Size: %d bytes\n", getVolume.Size)
	fmt.Printf("  Mount Point: %s\n", getVolume.MountPoint)
	fmt.Printf("  FileSystem Path: %s\n", getVolume.FileSystemPath)
	
	// Parse and display the server IP and mount point details
	if getVolume.FileSystemPath != "" {
		parts := strings.Split(getVolume.FileSystemPath, ":")
		if len(parts) >= 2 {
			serverIP := parts[0]
			mountPath := strings.Join(parts[1:], ":")
			fmt.Printf("\n  Mount Details:\n")
			fmt.Printf("  • Server IP: %s\n", serverIP)
			fmt.Printf("  • Mount Path: %s\n", mountPath)
			fmt.Printf("  • NFS Mount Command: mount -t nfs %s:%s /mnt/target\n", serverIP, mountPath)
		}
	}

	// Step 5: List volumes again (after creation)
	fmt.Println("\n5. Listing volumes (after creation)")
	volumes, err = client.ListVolumes(context.Background())
	if err != nil {
		fmt.Printf("ERROR: Failed to list volumes: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("✓ Found %d volumes\n", len(volumes))
	for i, vol := range volumes {
		fmt.Printf("  %d. Volume ID: %s, Name: %s, Type: %s, State: %s\n", 
			i+1, vol.ID, vol.Name, vol.Type, vol.State)
	}

	// Step 6: Delete the test volume
	fmt.Printf("\n6. Would you like to delete the test volume? (y/n): ")
	var answer string
	fmt.Scanln(&answer)
	
	if answer == "y" || answer == "Y" {
		fmt.Printf("Deleting volume: %s\n", newVolume.ID)
		err = client.DeleteVolume(context.Background(), newVolume.ID)
		if err != nil {
			fmt.Printf("ERROR: Failed to delete volume: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("✓ Volume deleted successfully!")

		// Verify volume was deleted
		fmt.Println("\n7. Listing volumes (after deletion)")
		volumes, err = client.ListVolumes(context.Background())
		if err != nil {
			fmt.Printf("ERROR: Failed to list volumes: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓ Found %d volumes\n", len(volumes))
		
		volumeFound := false
		for _, vol := range volumes {
			if vol.ID == newVolume.ID {
				volumeFound = true
				break
			}
		}
		
		if volumeFound {
			fmt.Println("WARNING: Volume still exists in the list (it may be in the process of being deleted)")
		} else {
			fmt.Println("✓ Volume has been removed from the list")
		}
	} else {
		fmt.Println("Skipping volume deletion. You can delete it manually using the Triton CLI:")
		fmt.Printf("triton volume delete %s\n", newVolume.ID)
	}

	fmt.Println("\n=== Test completed successfully ===")
}
