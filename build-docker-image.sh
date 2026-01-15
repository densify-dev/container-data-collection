#!/bin/bash

exec=$(basename "$0")

usage() {
    echo "" >&2
    echo "usage: ${exec} [ -b baseImage ] [ -t imageTag ] [ -a arch ] [ -p ] [ -r ] [ -h ]" >&2
    echo "" >&2
    echo "  b - alpine, ubi10, debian [ default is alpine ] " >&2
    echo "  t - required image tag [ mandatory ] " >&2
    echo "  a - target architecture: amd64, arm64, or multi [ default is current platform ] " >&2
    echo "  p - tag & push image to a registry repo [ mandatory for multi-arch ] " >&2
    echo "  i - registry is insecure [ default is false ] " >&2
    echo "  h - print help and exit " >&2
    echo "" >&2
    echo "Examples:" >&2
    echo "  ${exec} -t v4.2.2                           # Build for current platform" >&2
    echo "  ${exec} -t v4.2.2 -a amd64                  # Build for amd64 only" >&2
    echo "  ${exec} -t v4.2.2 -a arm64                  # Build for arm64 only" >&2
    echo "  ${exec} -t v4.2.2 -a multi -p <registry>    # Build multi-arch and push" >&2
    echo "  ${exec} -t v4.2.2 -a multi -p <registry> -i # Build multi-arch and push to an insecure registry" >&2
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

insecure=false
buildkitConfig=./build/buildkitd-config.toml

setupBuildx() {
    # Check if buildx is available
    if ! docker buildx version >/dev/null 2>&1; then
        echo "Docker Buildx is not available. Please install Docker Desktop or enable buildx."
        exit 1
    fi
    createArgs="--name multiarch-builder --driver docker-container --driver-opt network=host --bootstrap"
    if [ "${insecure}" = true ]; then
        createArgs="${createArgs} --buildkitd-config ${buildkitConfig}"
    fi
    createArgs="${createArgs} --use"
    # Create and use a new builder instance if it doesn't exist
    if ! docker buildx inspect multiarch-builder >/dev/null 2>&1; then
        echo "Creating multiarch-builder instance..."
        docker buildx create ${createArgs}
    else
        echo "Using multiarch-builder instance..."
        docker buildx use multiarch-builder
    fi
}

baseImageArg="alpine"
tag=""
arch=""
registryRepo=

while getopts 'b:t:a:p:ih' opt; do
    case $opt in
    # general options
    b) baseImageArg=$OPTARG ;;
    t) tag=$OPTARG ;;
    a) arch=$OPTARG ;;
    p) registryRepo=$OPTARG ;;
    i) insecure=true ;;
    # user asked for help, only case usage is called with 0
    h) usage 0 ;;
    # wrong option - usage error
    *) usage 1 ;;
    esac
done

rm -rf ./build
mkdir ./build

if [ -z "${tag}" ]; then
    usage 1
fi

if [ "${baseImageArg}" == "ubi10" ]; then
    : "${PYXIS_API_TOKEN:?Variable not set. Export PYXIS_API_TOKEN}"
    : "${RH_COMPONENT_ID:?Variable not set. Export RH_COMPONENT_ID}"
    baseImage="registry.access.redhat.com/ubi10/ubi-minimal"
    registryRepo="densify"
else
    baseImage="${baseImageArg}"
fi
image="container-data-collection-forwarder"

if [ -n ${registryRepo} ]; then
    registry=$(echo ${registryRepo} | cut -d'/' -f1)
    if [ ${insecure} == "true" ]; then
        cat << EOF > ${buildkitConfig}
[registry."${registry}"]
  http = true
  insecure = true
EOF
    fi
fi

# Validate architecture argument
case "${arch}" in
    ""|"amd64"|"arm64")
        ;;
    "multi")
        if [ -z "${registryRepo}" ]; then
            echo "Error: multi-arch build requires push to a registry repo"
            usage 1
        fi
        ;;
    *) 
        echo "Error: Invalid architecture '${arch}'. Valid options are: amd64, arm64, multi" >&2
        usage 1
        ;;
esac

isMulti=false
usesBuildx=true
# Set platform based on architecture argument
if [ "${arch}" = "multi" ]; then
    arch="amd64 arm64"
    platforms="linux/amd64,linux/arm64"
    isMulti=true
elif [ "${arch}" = "amd64" ]; then
    platforms="linux/amd64"
elif [ "${arch}" = "arm64" ]; then
    platforms="linux/arm64"
else
    # Default to current platform using regular docker build
    arch="amd64 arm64"
    platforms=""
    usesBuildx=false
fi

release=$(gitCommitHash)

# Go build as cross-compiling under docker buildx is notoriously slow
for trgArch in ${arch}; do
    mkdir -p ./build/${trgArch}
    GOOS=linux GOARCH=${trgArch} CGO_ENABLED=0 go build -trimpath \
        -gcflags=-trimpath="${GOPATH}" -asmflags=-trimpath="${GOPATH}" \
        -ldflags="-w -s" -o ./build/${trgArch}/dataCollection ./cmd
done

# build the image
echo "Building image for platforms: ${platforms:-"current platform"}"
docker pull golang:bookworm
docker pull ${baseImage}:latest

if [ "${usesBuildx}" = true ]; then
    setupBuildx
    # Use buildx for multi-platform builds
    buildArgs="--platform ${platforms}"
    buildArgs="${buildArgs} --provenance=false"
    buildArgs="${buildArgs} --build-arg BASE_IMAGE=${baseImage}"
    buildArgs="${buildArgs} --build-arg VERSION=${tag}"
    buildArgs="${buildArgs} --build-arg RELEASE=${release}"
    buildArgs="${buildArgs} -f Dockerfile-local"
    if [ -n "${registryRepo}" ]; then
        echo "Building and pushing multi-platform images..."
        buildArgs="${buildArgs} -t ${registryRepo}/${image}:${baseImageArg}-${tag} --push"
    else
        echo "Building multi-platform image for local use..."
        buildArgs="${buildArgs} -t ${image}:${baseImageArg}-${tag} --load"
    fi
    DOCKER_BUILDKIT=1 docker buildx build ${buildArgs} .
    if [ $? -ne 0 ]; then
        echo "❌ Build/Push failed"
        exit 1
    fi
else
    # Use regular docker build for single platform
    docker build --progress=plain -t ${image}:${baseImageArg}-${tag} -f Dockerfile-local --build-arg BASE_IMAGE=${baseImage} --build-arg VERSION=${tag} --build-arg RELEASE=${release} .
    if [ $? -ne 0 ]; then
        echo "❌ Push failed"
        exit 1
    fi
    # Traditional push logic for single platform builds
    if [ -n "${registryRepo}" ]; then
        tagAndPush ${image}:${baseImageArg}-${tag} ${registryRepo}/${image}:${baseImageArg}-${tag}
    fi
    if [ $? -ne 0 ]; then
        echo "❌ Tag/Push failed"
        exit 1
    fi
fi

if [ "${baseImageArg}" == "ubi10" ]; then
    echo "--- 🕵️ Running Red Hat Preflight Certification ---"
    # Preflight automatically looks for credentials in ~/.docker/config.json
    # so we don't need to specify --docker-config explicitly unless it's non-standard.
    preflight check container "${registryRepo}/${image}:${baseImageArg}-${tag}" \
        --submit \
        --pyxis-api-token="${PYXIS_API_TOKEN}" \
        --certification-component-id="${RH_COMPONENT_ID}"

    if [ $? -eq 0 ]; then
        echo "✅ Success! Results submitted to Red Hat."
    else
        echo "❌ Preflight checks failed."
        exit 1
    fi
fi