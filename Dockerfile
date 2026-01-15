ARG BASE_IMAGE=alpine

FROM golang:bookworm AS builder
# Enable Docker BuildKit automatic platform ARGs
ARG TARGETARCH
ARG VERSION
ADD . /github.com/densify-dev/container-data-collection
WORKDIR /github.com/densify-dev/container-data-collection/cmd
RUN echo -n "v${VERSION}" > ../internal/common/version.txt
RUN GOOS=linux GOARCH=${TARGETARCH} CGO_ENABLED=0 go build -trimpath -gcflags=-trimpath="${GOPATH}" -asmflags=-trimpath="${GOPATH}" -ldflags="-w -s" -o ./dataCollection .

FROM ${BASE_IMAGE}:latest
ARG BASE_IMAGE
ENV BASE_IMG=${BASE_IMAGE}
ARG VERSION
ARG RELEASE
# Enable Docker BuildKit automatic platform ARGs for runtime stage
ARG TARGETARCH

### Required OpenShift Labels
LABEL name="Container-Optimization-Data-Forwarder" \
      vendor="Kubex" \
      maintainer="support@kubex.ai" \
      version="${VERSION}" \
      release="${RELEASE}" \
      summary="Kubex container data collection" \
      description="Collects data from Prometheus and sends to Kubex server for analysis"

# BASE_IMAGE specifics - add non-root user and remove the ability to install packages
RUN case ${BASE_IMG} in \
    alpine* ) \
        addgroup -g 3000 densify && \
        adduser -h /home/densify -s /bin/sh -u 3000 -G densify -g "" -D densify && \
        rm -f /sbin/apk ;; \
    *ubi* ) \
        microdnf install -y shadow-utils && \
        groupadd -g 3000 densify && \
        adduser -d /home/densify -m -s /bin/bash -u 3000 -g densify densify && \
        microdnf remove -y shadow-utils && \
        microdnf clean all && \
        rm -f /bin/microdnf ;; \
    debian* ) \
        mkdir -p /home/densify && \
        addgroup --gid 3000 densify && \
        useradd --home /home/densify --shell /bin/bash --uid 3000 --gid 3000 --password "" densify && \
        chown densify:densify /home/densify && \
        rm -f /usr/bin/apt /usr/bin/apt-get ;; \
    * ) \
        echo "unsupported base image ${BASE_IMG}" && \
        exit 1 ;; \
    esac

### make sure /home/densify is accessible
RUN chmod 755 /home/densify

### add licenses to this directory
COPY --chown=densify:densify --chmod=644 ./LICENSE /licenses/LICENSE
### keep /config as this is how it is mounted in versions < 3.0
RUN mkdir /config

WORKDIR /home/densify
RUN mkdir -p data && chown -R densify:densify /home/densify/data && chmod -R 777 /home/densify/data && ln -s /config config
COPY --chown=densify:densify --chmod=755 ./tools/${TARGETARCH}/forwarder ./tools/entry.sh bin/
COPY --chown=densify:densify --chmod=755 --from=builder /github.com/densify-dev/container-data-collection/cmd/dataCollection bin/
USER 3000
CMD ["/home/densify/bin/entry.sh"]
