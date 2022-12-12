# Copyright IBM Corp. All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

ARG GO_VER
FROM golang:${GO_VER}-bullseye as golang

RUN apt-get update && apt-get install -y\
	bash \
	binutils-gold \
	gcc \
	git \
	make \
	musl-dev && \
	rm -rf /var/lib/apt/lists/*

ADD . $GOPATH/src/github.com/hyperledger/fabric
WORKDIR $GOPATH/src/github.com/hyperledger/fabric

FROM golang as tools
ARG GO_TAGS
RUN make tools GO_TAGS=${GO_TAGS}

FROM golang:${GO_VER}-bullseye
ARG YUBIHSM_VERSION="2022-06-debian11"
ARG YUBIHSM_PACKAGE_VERSION="2.3.2"
ARG DEBIAN_FRONTEND="noninteractive"
# git is required to support `go list -m`
RUN apt-get update && apt-get install -y\
	bash \
	git \
	jq \
	tzdata \
	libusb-1.0-0 \
	softhsm && \
	cd /tmp && \
	curl -LO https://developers.yubico.com/YubiHSM2/Releases/yubihsm2-sdk-${YUBIHSM_VERSION}-amd64.tar.gz && \
	tar xzf yubihsm2-sdk-${YUBIHSM_VERSION}-amd64.tar.gz && \
	cd /tmp/yubihsm2-sdk && \
	dpkg -i libyubihsm-http1_${YUBIHSM_PACKAGE_VERSION}_amd64.deb \
	libyubihsm-usb1_${YUBIHSM_PACKAGE_VERSION}_amd64.deb \
	libyubihsm1_${YUBIHSM_PACKAGE_VERSION}_amd64.deb \
	yubihsm-pkcs11_${YUBIHSM_PACKAGE_VERSION}_amd64.deb && \
	cd / && \
	rm -rf tmp/yubihsm2-sdk-${YUBIHSM_VERSION}-amd64.tar.gz tmp/yubihsm2-sdk /var/lib/apt/lists/*

ENV FABRIC_CFG_PATH /etc/hyperledger/fabric
VOLUME /etc/hyperledger/fabric
COPY --from=tools /go/src/github.com/hyperledger/fabric/build/bin /usr/local/bin
COPY --from=tools /go/src/github.com/hyperledger/fabric/sampleconfig ${FABRIC_CFG_PATH}
