#!/bin/bash

set -e

CONTAINER_NAME="featx-memory"
IMAGE_NAME="redis:latest"

echo "========================================"
echo " Redis Docker Container Setup"
echo "========================================"

# ----------------------------------------
# 1. Check if Docker is installed
# ----------------------------------------
echo ""
echo "Checking Docker installation..."

if ! command -v docker &> /dev/null; then
    echo "ERROR: Docker is not installed."
    exit 1
fi

echo "Docker is installed."

# ----------------------------------------
# 2. Check if Docker daemon is running
# ----------------------------------------
echo ""
echo "Checking Docker daemon..."

if ! docker info &> /dev/null; then
    echo "Docker daemon is not running."
    echo "Starting Docker daemon..."

    sudo systemctl start docker

    sleep 2

    if ! docker info &> /dev/null; then
        echo "ERROR: Failed to start Docker daemon."
        exit 1
    fi

    echo "Docker daemon started successfully."
else
    echo "Docker daemon is already running."
fi

# ----------------------------------------
# 3. Check if Redis image exists
# ----------------------------------------
echo ""
echo "Checking Redis Docker image..."

if docker image inspect "$IMAGE_NAME" &> /dev/null; then
    echo "Redis image '$IMAGE_NAME' already exists locally."
else
    echo "Redis image '$IMAGE_NAME' was not found locally."
    echo ""
    echo "Downloading official Redis image..."
    echo "Please wait. Docker will display download progress below."
    echo ""

    docker pull "$IMAGE_NAME"

    echo ""
    echo "Redis image downloaded successfully."
fi

# ----------------------------------------
# 4. Check if Redis container exists
# ----------------------------------------
echo ""
echo "Checking Redis container..."

if docker ps -a --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then

    echo "Container '$CONTAINER_NAME' already exists."

    # Check if container is already running
    if docker ps --format '{{.Names}}' | grep -q "^${CONTAINER_NAME}$"; then
        echo "Redis container is already running."
    else
        echo "Redis container is stopped."
        echo "Starting Redis container..."

        docker start "$CONTAINER_NAME"

        echo "Redis container started successfully."
    fi

else

    echo "Container '$CONTAINER_NAME' does not exist."
    echo "Creating and starting Redis container..."

    docker run -d \
        --name "$CONTAINER_NAME" \
        "$IMAGE_NAME"

    echo "Redis container created and started successfully."

fi

# ----------------------------------------
# 5. Display final status
# ----------------------------------------
echo ""
echo "========================================"
echo " Redis container status"
echo "========================================"

docker ps --filter "name=$CONTAINER_NAME"

echo ""
echo "Setup completed."