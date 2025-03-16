#!/bin/bash
set -e

# Get new version from VERSION file
NEW_VERSION=$(cat VERSION)
echo "Updating to version: $NEW_VERSION"

# Update driver.go hardcoded version
sed -i'' -e "s/DriverVersion = \"v[0-9]\+\.[0-9]\+\.[0-9]\+\"/DriverVersion = \"$NEW_VERSION\"/" pkg/driver/driver.go

# Update Dockerfile ARG VERSION
sed -i'' -e "s/ARG VERSION=v[0-9]\+\.[0-9]\+\.[0-9]\+/ARG VERSION=$NEW_VERSION/" Dockerfile

# Update deployment YAML files
find deploy examples -name "*.yaml" -type f -exec sed -i'' -e "s/image: nwilkens\/tritonnfs-csi:v[0-9]\+\.[0-9]\+\.[0-9]\+/image: nwilkens\/tritonnfs-csi:$NEW_VERSION/g" {} \;

# Check for any other YAML files at the root that might need updating
find . -maxdepth 1 -name "*-*.yaml" -type f -exec sed -i'' -e "s/image: nwilkens\/tritonnfs-csi:v[0-9]\+\.[0-9]\+\.[0-9]\+/image: nwilkens\/tritonnfs-csi:$NEW_VERSION/g" {} \;

echo "Version updated in all files to $NEW_VERSION"
echo "Now run 'make docker-build docker-push' to build and push the new image"