#!/usr/bin/env bash
set -euo pipefail

# Build and push this project image to GitHub Container Registry (ghcr.io)
# Usage:
#   ./push-ghcr.sh <tag1> [tag2 ...]
#
# Optional env vars:
#   IMAGE_NAME        Full image name (default: ghcr.io/<owner>/qad)
#   DOCKERFILE_PATH   Dockerfile path (default: ./Dockerfile)
#   CONTEXT_PATH      Build context (default: .)
#
# Example:
#   IMAGE_NAME=ghcr.io/my-org/qad ./push-ghcr.sh v1.0.0 latest

if [[ $# -lt 1 ]]; then
  echo "Usage: $0 <tag1> [tag2 ...]"
  exit 1
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "Error: docker is not installed or not in PATH"
  exit 1
fi

DEFAULT_OWNER="${GITHUB_REPOSITORY_OWNER:-${GITHUB_ACTOR:-}}"
if [[ -z "${DEFAULT_OWNER}" ]]; then
  DEFAULT_OWNER="habetuz"
fi
DEFAULT_OWNER="$(echo "${DEFAULT_OWNER}" | tr '[:upper:]' '[:lower:]')"

IMAGE_NAME="${IMAGE_NAME:-ghcr.io/${DEFAULT_OWNER}/qad}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-./Dockerfile}"
CONTEXT_PATH="${CONTEXT_PATH:-.}"

if [[ ! -f "${DOCKERFILE_PATH}" ]]; then
  echo "Error: Dockerfile not found at ${DOCKERFILE_PATH}"
  exit 1
fi

# Check if user is logged into ghcr.io
if ! docker info 2>/dev/null | grep -q "Username:"; then
  echo "Warning: Docker may not be logged in."
  echo "Run: echo \"<GITHUB_TOKEN>\" | docker login ghcr.io -u <github-user> --password-stdin"
fi

FIRST_TAG="$1"
FIRST_REF="${IMAGE_NAME}:${FIRST_TAG}"

echo "Building ${FIRST_REF} ..."
docker build -f "${DOCKERFILE_PATH}" -t "${FIRST_REF}" "${CONTEXT_PATH}"

for TAG in "$@"; do
  REF="${IMAGE_NAME}:${TAG}"

  if [[ "${TAG}" != "${FIRST_TAG}" ]]; then
    echo "Tagging ${FIRST_REF} as ${REF} ..."
    docker tag "${FIRST_REF}" "${REF}"
  fi

  echo "Pushing ${REF} ..."
  docker push "${REF}"
done

echo "Done. Pushed ${#} tag(s) to ${IMAGE_NAME}."
