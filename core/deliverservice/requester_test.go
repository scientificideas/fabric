/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice

import (
	"testing"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/stretchr/testify/assert"
)

func TestRequestByContent(t *testing.T) {
	type testCase struct {
		name        string
		contentType orderer.SeekInfo_SeekContentType
		height      uint64
	}

	testcases := []testCase{
		{"block-0", orderer.SeekInfo_BLOCK, 0},
		{"block-100", orderer.SeekInfo_BLOCK, 100},
		{"header-0", orderer.SeekInfo_HEADER_WITH_SIG, 0},
		{"header-100", orderer.SeekInfo_HEADER_WITH_SIG, 100},
	}

	for _, tc := range testcases {
		t.Run(tc.name, func(t *testing.T) {
			requester := blocksRequester{
				chainID:           "testchainid",
				signer:            nil,
				deliverGPRCClient: nil,
			}

			var (
				envelope *common.Envelope
				err      error
			)

			if tc.contentType == orderer.SeekInfo_BLOCK {
				envelope, err = requester.RequestBlocks(tc.height)
			} else {
				envelope, err = requester.RequestHeaders(tc.height)
			}
			assert.NoError(t, err)
			assert.NotNil(t, envelope)
			assert.NotNil(t, envelope.Payload)
			assert.Nil(t, envelope.Signature)
		})
	}
}
