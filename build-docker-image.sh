#!/bin/bash

exec=$(basename "$0")

usage() {
    echo "" >&2
    echo "usage: ${exec} [ -b baseImage ] [ -t imageTag ] [ -a arch ] [ -p ] [ -r ] [ -h ]" >&2
    echo "" >&2
    echo "  b - alpine, ubi9, debian [ default is alpine ] " >&2
    echo "  t - required image tag [ mandatory ] " >&2
    echo "  a - target architecture: amd64, arm64, or multi [ default is current platform ] " >&2
    echo "  o - official release image (implied tagging) " >&2
    echo "  p - tag & push image to quay.io and Docker hub " >&2
    echo "  h - print help and exit " >&2
    echo "" >&2
    echo "Examples:" >&2
    echo "  ${exec} -t v4.2.2                    # Build for current platform" >&2
    echo "  ${exec} -t v4.2.2 -a amd64          # Build for amd64 only" >&2
    echo "  ${exec} -t v4.2.2 -a arm64          # Build for arm64 only" >&2
    echo "  ${exec} -t v4.2.2 -a multi          # Build for both amd64 and arm64" >&2
    echo "  ${exec} -t v4.2.2 -a multi -p       # Build multi-arch and push" >&2
    echo "" >&2

    exit $1
}

tagAndPush() {
    action="tag $1 $2"
    docker ${action}
    rc=$?
    if [ $rc -eq 0 ]; then
        action="push $2"
        docker ${action}
        rc=$?
    fi
    if [ $rc -ne 0 ]; then
        echo "docker ${action} failed with return code $rc, exiting"
        exit $rc
    fi
}

gitCommitHash() {
    git rev-parse --verify HEAD
    rc=$?
    if [ $rc -ne 0 ]; then
        echo "failed to get git commit hash, exiting"
        exit $rc
    fi
}

setupBuildx() {
    # Check if buildx is available
    if ! docker buildx version >/dev/null 2>&1; then
        echo "Docker Buildx is not available. Please install Docker Desktop or enable buildx."
        exit 1
    fi
    
    # Create and use a new builder instance if it doesn't exist
    if ! docker buildx inspect multiarch-builder >/dev/null 2>&1; then
        echo "Creating multiarch-builder instance..."
        docker buildx create --name multiarch-builder --driver docker-container --bootstrap
    fi
    
    echo "Using multiarch-builder instance..."
    docker buildx use multiarch-builder
}

baseImageArg="alpine"
tag=""
arch=""
push=0
official=0

while getopts 'b:t:a:oph' opt; do
    case $opt in
    # general options
    b) baseImageArg=$OPTARG ;;
    t) tag=$OPTARG ;;
    a) arch=$OPTARG ;;
    o) official=1 ;;
    p) push=1 ;;
    # user asked for help, only case usage is called with 0
    h) usage 0 ;;
    # wrong option - usage error
    *) usage 1 ;;
    esac
done

if [ -z "${tag}" ]; then
    usage 1
fi

# Validate architecture argument
case "${arch}" in
    ""|"amd64"|"arm64"|"multi") ;;
    *) 
        echo "Error: Invalid architecture '${arch}'. Valid options are: amd64, arm64, multi" >&2
        usage 1
        ;;
esac

# Set platform based on architecture argument
if [ "${arch}" = "multi" ]; then
    platforms="linux/amd64,linux/arm64"
    usesBuildx=true
elif [ "${arch}" = "amd64" ]; then
    platforms="linux/amd64"
    usesBuildx=true
elif [ "${arch}" = "arm64" ]; then
    platforms="linux/arm64"
    usesBuildx=true
else
    # Default to current platform using regular docker build
    platforms=""
    usesBuildx=false
fi

# full name of ubi9 image
if [ "${baseImageArg}" == "ubi9" ]; then
    baseImage="redhat/ubi9-minimal"
else
    baseImage="${baseImageArg}"
fi

quayImage="container-data-collection-forwarder"
quayRepo="quay.io/densify/"
dockerHubImage="container-optimization-data-forwarder"
dockerHubRepo="densify/"

release=$(gitCommitHash)

# Setup buildx if needed
if [ "${usesBuildx}" = true ]; then
    setupBuildx
fi

# build the image
echo "Building image for platforms: ${platforms:-"current platform"}"

if [ "${usesBuildx}" = true ]; then
    # Use buildx for multi-platform builds
    buildArgs="--platform ${platforms}"
    buildArgs="${buildArgs} --build-arg BASE_IMAGE=${baseImage}"
    buildArgs="${buildArgs} --build-arg VERSION=${tag}"
    buildArgs="${buildArgs} --build-arg RELEASE=${release}"
    buildArgs="${buildArgs} -f Dockerfile"
    buildArgs="${buildArgs} -t ${quayImage}:${baseImageArg}-${tag}"
    
    if [ ${push} -eq 1 ]; then
        # Push to registries during build
        buildArgs="${buildArgs} --push"
        # Add additional tags for pushing
        buildArgs="${buildArgs} -t ${quayRepo}${quayImage}:${baseImageArg}-${tag}"
        if [ "${baseImageArg}" = "alpine" ]; then
            buildArgs="${buildArgs} -t ${dockerHubRepo}${dockerHubImage}:${baseImageArg}-${tag}"
        fi
        if [ ${official} -eq 1 ]; then
            buildArgs="${buildArgs} -t ${quayRepo}${quayImage}:${baseImageArg}"
            if [ "${baseImageArg}" = "alpine" ]; then
                buildArgs="${buildArgs} -t ${quayRepo}${quayImage}:latest"
                buildArgs="${buildArgs} -t ${dockerHubRepo}${dockerHubImage}:${baseImageArg}"
                buildArgs="${buildArgs} -t ${dockerHubRepo}${dockerHubImage}:latest"
            fi
        fi
        echo "Building and pushing multi-platform images..."
        docker buildx build ${buildArgs} .
    else
        # Load for local use
        buildArgs="${buildArgs} --load"
        echo "Building multi-platform image for local use..."
        docker buildx build ${buildArgs} .
    fi
else
    # Use regular docker build for single platform
    docker pull golang:bookworm
    docker pull ${baseImage}:latest
    docker build --progress=plain -t ${quayImage}:${baseImageArg}-${tag} -f Dockerfile --build-arg BASE_IMAGE=${baseImage} --build-arg VERSION=${tag} --build-arg RELEASE=${release} .
    
    # Traditional push logic for single platform builds
    if [ ${push} -eq 1 ]; then
        tagAndPush ${quayImage}:${baseImageArg}-${tag} ${quayRepo}${quayImage}:${baseImageArg}-${tag}
        if [ "${baseImageArg}" == "alpine" ]; then
            tagAndPush ${quayImage}:${baseImageArg}-${tag} ${dockerHubRepo}${dockerHubImage}:${baseImageArg}-${tag}
        fi
        if [ ${official} -eq 1 ]; then
            tagAndPush ${quayImage}:${baseImageArg}-${tag} ${quayRepo}${quayImage}:${baseImageArg}
            if [ "${baseImageArg}" == "alpine" ]; then
                tagAndPush ${quayImage}:${baseImageArg}-${tag} ${quayRepo}${quayImage}:latest
                tagAndPush ${quayImage}:${baseImageArg}-${tag} ${dockerHubRepo}${dockerHubImage}:${baseImageArg}
                tagAndPush ${quayImage}:${baseImageArg}-${tag} ${dockerHubRepo}${dockerHubImage}:latest
            fi
        fi
    fi
fi
