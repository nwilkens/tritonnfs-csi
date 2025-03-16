FROM --platform=linux/amd64 golang:1.21 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

ARG VERSION=v0.6.0
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -ldflags "-X github.com/joyent/tritonnfs-csi/pkg/driver.DriverVersion=${VERSION}" -o tritonnfs-csi cmd/tritonnfs-csi/main.go

FROM --platform=linux/amd64 ubuntu:20.04
WORKDIR /

# Install NFS utilities with all dependencies
RUN apt-get update && \
    DEBIAN_FRONTEND=noninteractive apt-get install -y \
    nfs-common \
    rpcbind \
    nfs-kernel-server \
    nfswatch \
    iputils-ping \
    procps \
    net-tools \
    dnsutils \
    ca-certificates && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*

# Create startup script that ensures NFS services are running
RUN echo '#!/bin/bash\n\
echo "Starting NFS services..."\n\
\n\
# Make sure rpcbind is running\n\
rpcbind || echo "Failed to start rpcbind"\n\
\n\
# Load NFS kernel modules if possible\n\
mkdir -p /lib/modules\n\
echo "Checking NFS modules:"\n\
modprobe -v nfs || echo "NFS module not available in container (this is expected, host should have it)"\n\
modprobe -v nfsd || echo "NFSD module not available in container (this is expected, host should have it)"\n\
\n\
# Check NFS status\n\
echo "NFS status:"\n\
rpcinfo -p localhost || echo "RPC services not available"\n\
\n\
# Show mount capabilities\n\
echo "Mount capabilities:"\n\
mount -V\n\
\n\
# Start the CSI driver\n\
echo "Starting CSI driver..."\n\
exec /tritonnfs-csi "$@"\n\
' > /start.sh && chmod +x /start.sh

COPY --from=builder /workspace/tritonnfs-csi /tritonnfs-csi

# Test the NFS configuration
RUN echo "Testing NFS configuration:" && \
    ls -la /sbin/mount* /bin/mount* && \
    which showmount && \
    showmount --version && \
    mount -V

ENTRYPOINT ["/start.sh"]