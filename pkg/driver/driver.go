package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"k8s.io/mount-utils"
)

var (
	// DriverVersion is the version of the CSI driver
	// This is set via ldflags during build
	DriverVersion = "v0.5.5" // Default value, will be overridden during build
)

// TritonNFSDriver implements the CSI driver interface for Triton NFS volumes
type TritonNFSDriver struct {
	endpoint   string
	nodeID     string
	cloudAPI   string
	accountID  string
	keyID      string
	keyPath    string
	server     *grpc.Server
	mounter    mount.Interface
	tritonClient *TritonClient
}

// DriverOption is a functional option for configuring the driver
type DriverOption func(*TritonNFSDriver) error

// WithEndpoint sets the endpoint for the driver
func WithEndpoint(endpoint string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.endpoint = endpoint
		return nil
	}
}

// WithNodeID sets the node ID for the driver
func WithNodeID(nodeID string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.nodeID = nodeID
		return nil
	}
}

// WithCloudAPI sets the CloudAPI endpoint for the driver
func WithCloudAPI(cloudAPI string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.cloudAPI = cloudAPI
		return nil
	}
}

// WithAccountID sets the account ID for the driver
func WithAccountID(accountID string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.accountID = accountID
		return nil
	}
}

// WithKeyID sets the key ID for the driver
func WithKeyID(keyID string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.keyID = keyID
		return nil
	}
}

// WithKeyPath sets the key path for the driver
func WithKeyPath(keyPath string) DriverOption {
	return func(driver *TritonNFSDriver) error {
		driver.keyPath = keyPath
		return nil
	}
}

// NewTritonNFSDriver creates a new TritonNFSDriver with the given options
func NewTritonNFSDriver(opts ...DriverOption) (*TritonNFSDriver, error) {
	driver := &TritonNFSDriver{
		mounter: mount.New(""),
	}

	for _, opt := range opts {
		if err := opt(driver); err != nil {
			return nil, err
		}
	}

	tritonClient, err := NewTritonClient(driver.cloudAPI, driver.accountID, driver.keyID, driver.keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create Triton client: %v", err)
	}
	driver.tritonClient = tritonClient

	return driver, nil
}

// Run starts the CSI driver
func (d *TritonNFSDriver) Run() error {
	scheme, addr, err := parseEndpoint(d.endpoint)
	if err != nil {
		return err
	}

	logrus.Infof("Starting Triton NFS CSI driver version %s at %s", DriverVersion, d.endpoint)
	listener, err := net.Listen(scheme, addr)
	if err != nil {
		return err
	}

	logrus.Infof("Listening for connections on address: %#v", listener.Addr())

	opts := []grpc.ServerOption{
		grpc.UnaryInterceptor(logGRPC),
	}
	d.server = grpc.NewServer(opts...)

	csi.RegisterIdentityServer(d.server, d)
	csi.RegisterControllerServer(d.server, d)
	csi.RegisterNodeServer(d.server, d)

	return d.server.Serve(listener)
}

// Stop stops the CSI driver
func (d *TritonNFSDriver) Stop() {
	if d.server != nil {
		d.server.Stop()
	}
}

func parseEndpoint(endpoint string) (string, string, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return "", "", fmt.Errorf("could not parse endpoint: %v", err)
	}

	var addr string
	if u.Scheme == "unix" {
		addr = u.Path
		if err := os.Remove(addr); err != nil && !os.IsNotExist(err) {
			return "", "", fmt.Errorf("could not remove unix domain socket %q: %v", addr, err)
		}
		socketDir := filepath.Dir(addr)
		if err := os.MkdirAll(socketDir, 0755); err != nil {
			return "", "", fmt.Errorf("could not create directory %q: %v", socketDir, err)
		}
	} else {
		addr = u.Host
	}

	return u.Scheme, addr, nil
}

func logGRPC(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	logrus.Debugf("GRPC call: %s", info.FullMethod)
	logrus.Debugf("GRPC request: %s", protosanitizer.StripSecrets(req))
	resp, err := handler(ctx, req)
	if err != nil {
		logrus.Errorf("GRPC error: %v", err)
	} else {
		logrus.Debugf("GRPC response: %s", protosanitizer.StripSecrets(resp))
	}
	return resp, err
}