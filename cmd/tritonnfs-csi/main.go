package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joyent/tritonnfs-csi/pkg/driver"
	"github.com/sirupsen/logrus"
)

var (
	endpoint   = flag.String("endpoint", "unix:///var/lib/kubelet/plugins/tritonnfs.csi.triton.com/csi.sock", "CSI endpoint")
	driverName = flag.String("driver-name", "tritonnfs.csi.triton.com", "Name of the driver")
	nodeID     = flag.String("node-id", "", "Node ID")
	version    = flag.Bool("version", false, "Print the version and exit")
	cloudAPI   = flag.String("cloud-api", "", "Triton CloudAPI endpoint")
	accountID  = flag.String("account-id", "", "Triton account ID")
	keyID      = flag.String("key-id", "", "Triton key ID")
	keyPath    = flag.String("key-path", "", "Path to Triton private key file")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Printf("TritonNFS CSI Driver Version: %s\n", driver.DriverVersion)
		os.Exit(0)
	}

	if *nodeID == "" {
		logrus.Fatal("node-id is required")
	}

	if *cloudAPI == "" {
		logrus.Fatal("cloud-api endpoint is required")
	}

	if *accountID == "" {
		logrus.Fatal("account-id is required")
	}

	if *keyID == "" {
		logrus.Fatal("key-id is required")
	}

	if *keyPath == "" {
		logrus.Fatal("key-path is required")
	}

	logrus.Infof("Starting TritonNFS CSI driver: %s version: %s", *driverName, driver.DriverVersion)

	drv, err := driver.NewTritonNFSDriver(
		driver.WithEndpoint(*endpoint),
		driver.WithNodeID(*nodeID),
		driver.WithCloudAPI(*cloudAPI),
		driver.WithAccountID(*accountID),
		driver.WithKeyID(*keyID),
		driver.WithKeyPath(*keyPath),
	)
	if err != nil {
		logrus.Fatalf("Failed to create TritonNFS CSI driver: %v", err)
	}

	err = drv.Run()
	if err != nil {
		logrus.Fatalf("Failed to run TritonNFS CSI driver: %v", err)
	}
}