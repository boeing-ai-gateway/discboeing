#!/bin/bash
# Load preloaded OCI image into Docker if it doesn't already exist.
# The image tarball and tag are baked into the VM image at build time.
set -e

IMAGE_TAG_FILE="/preload-image-tag"
IMAGE_TARBALL="/preload-image.tar"

if [ ! -f "$IMAGE_TAG_FILE" ] || [ ! -f "$IMAGE_TARBALL" ]; then
    echo "No preloaded image found, skipping"
    exit 0
fi

IMAGE_TAG=$(cat "$IMAGE_TAG_FILE")
echo "Checking for preloaded image: ${IMAGE_TAG}"

# Check if the image already exists in Docker (e.g. from a previous boot)
if docker image inspect "${IMAGE_TAG}" >/dev/null 2>&1; then
    echo "Image ${IMAGE_TAG} already exists, skipping load"
    exit 0
fi

echo "Loading preloaded image ${IMAGE_TAG} into Docker..."
docker load < "$IMAGE_TARBALL"
echo "Preloaded image ${IMAGE_TAG} loaded successfully"
