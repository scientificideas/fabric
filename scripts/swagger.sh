#!/bin/bash -e

# Copyright IBM Corp All Rights Reserved.
#
# SPDX-License-Identifier: Apache-2.0

fabric_dir="$(cd "$(dirname "$0")/.." && pwd)"
swagger_orderer_tags="${fabric_dir}/swagger/tags.json"
swagger_orderer_doc="${fabric_dir}/swagger/swagger-orderer-fabric.json"

swagger_peer_rest="${fabric_dir}/internal/peer/rest/pbrest/rest.swagger.json"
swagger_peer_doc="${fabric_dir}/swagger/swagger-peer-fabric.json"

check_spec() {
    swagger_orderer_doc_check="${fabric_dir}/swagger/swagger-orderer-fabric-check.json"
    swagger generate spec -o "$swagger_orderer_doc_check" --scan-models --exclude-deps --input "$swagger_orderer_tags"
    if [ -n "$(diff "$swagger_orderer_doc_check" "$swagger_orderer_doc")" ]; then
        echo "The Fabric orderer swagger is out of date."
        echo "Please run '$0 generate' to update the swagger."
        rm "$swagger_orderer_doc_check"
        exit 1
    fi
    rm "$swagger_orderer_doc_check"

    swagger_peer_doc_check="${fabric_dir}/swagger/swagger-peer-fabric-check.json"
        swagger generate spec -o "$swagger_peer_doc_check" --scan-models --exclude-deps --include-tag "operations" --exclude github.com/hyperledger/fabric/orderer/common/types --input "$swagger_peer_rest"
        if [ -n "$(diff "$swagger_peer_doc_check" "$swagger_peer_doc")" ]; then
            echo "The Fabric peer swagger is out of date."
            echo "Please run '$0 generate' to update the swagger."
            rm "$swagger_peer_doc_check"
            exit 1
        fi
        rm "$swagger_peer_doc_check"
}

case "$1" in
    # check if the swagger is up to date with the swagger
    # options in the tree
    "check")
        check_spec
    ;;

    # generate the swagger
    "generate")
        swagger generate spec -o "$swagger_orderer_doc" --scan-models --exclude-deps --input "$swagger_orderer_tags"
        swagger generate spec -o "$swagger_peer_doc" --scan-models --exclude-deps --include-tag "operations" --exclude github.com/hyperledger/fabric/orderer/common/types --input "$swagger_peer_rest"
    ;;

    *)
        echo "Please specify check or generate"
        exit 1
    ;;
esac

