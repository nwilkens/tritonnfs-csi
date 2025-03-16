FROM golang:1.21 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY cmd/ cmd/
COPY pkg/ pkg/

ARG VERSION=v0.5.5
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -ldflags "-X github.com/joyent/tritonnfs-csi/pkg/driver.DriverVersion=${VERSION}" -o tritonnfs-csi cmd/tritonnfs-csi/main.go

FROM debian:bullseye-slim
WORKDIR /
RUN apt-get update && \
    apt-get install -y nfs-common && \
    apt-get clean && \
    rm -rf /var/lib/apt/lists/*
COPY --from=builder /workspace/tritonnfs-csi /tritonnfs-csi

ENTRYPOINT ["/tritonnfs-csi"]