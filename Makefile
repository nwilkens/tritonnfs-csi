VERSION ?= $(shell cat VERSION)
REGISTRY ?= nwilkens
IMAGE_NAME ?= tritonnfs-csi
IMAGE_TAG ?= $(VERSION)
FULL_IMAGE_NAME = $(REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)

# Go build settings
GOOS ?= linux
GOARCH ?= amd64
BUILD_FLAGS ?= -a
LDFLAGS ?= -X github.com/nwilkens/tritonnfs-csi/pkg/driver.DriverVersion=$(VERSION)

.PHONY: all
all: build

.PHONY: build
build:
	CGO_ENABLED=0 GOOS=$(GOOS) GOARCH=$(GOARCH) GO111MODULE=on go build $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" -o bin/tritonnfs-csi cmd/tritonnfs-csi/main.go

.PHONY: test
test:
	go test -v ./...

.PHONY: clean
clean:
	rm -rf bin

.PHONY: docker-build
docker-build:
	docker build --build-arg VERSION=$(VERSION) -t $(FULL_IMAGE_NAME) .

.PHONY: docker-push
docker-push: docker-build
	docker push $(FULL_IMAGE_NAME)

.PHONY: version
version:
	@echo $(VERSION)

.PHONY: bump-version
bump-version:
	@echo "Updating all version references to $(VERSION)"
	./update-version.sh

.PHONY: help
help:
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@echo "  build           Build the binary locally"
	@echo "  test            Run tests"
	@echo "  clean           Remove build artifacts"
	@echo "  docker-build    Build docker image"
	@echo "  docker-push     Build and push docker image"
	@echo "  version         Display the current version"
	@echo "  bump-version    Update all version references from VERSION file"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION         Version (default: $(VERSION))"
	@echo "  REGISTRY        Docker registry to push to (default: $(REGISTRY))"
	@echo "  IMAGE_NAME      Name of the docker image (default: $(IMAGE_NAME))"
	@echo "  IMAGE_TAG       Tag of the docker image (default: $(VERSION))"
