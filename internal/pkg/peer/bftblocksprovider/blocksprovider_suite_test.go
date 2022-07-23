/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package bftblocksprovider_test

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/internal/pkg/peer/orderers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

//go:generate counterfeiter -o fake/orderer_connection_source.go --fake-name OrdererConnectionSource . ordererConnectionSource
type ordererConnectionSource interface {
	GetAllEndpoints() []*orderers.Endpoint
	InitUpdateEndpointsChannel() chan []*orderers.Endpoint
}

//go:generate counterfeiter -o fake/signer.go --fake-name Signer . signer
type signer interface {
	identity.SignerSerializer
}

//go:generate counterfeiter -o fake/ab_deliver_client.go --fake-name DeliverClient . abDeliverClient
type abDeliverClient interface {
	orderer.AtomicBroadcast_DeliverClient
}

func TestBlocksprovider(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Blocksprovider BFT Suite")
}
