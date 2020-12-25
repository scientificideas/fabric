/*
Copyright IBM Corp. 2017 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

                 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package deliverservice

import (
	"math"

	"github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"github.com/hyperledger/fabric/protoutil"
)

type blocksRequester struct {
	chainID           string
	signer            identity.SignerSerializer
	deliverGPRCClient *comm.GRPCClient
}

func (b *blocksRequester) RequestBlocks(height uint64) (*common.Envelope, error) {
	if height > 0 {
		logger.Infof("Starting deliver with block [%d] for channel %s", height, b.chainID)
		return b.seekLatestFromCommitter(height, orderer.SeekInfo_BLOCK)
	}

	logger.Infof("Starting deliver with oldest block for channel %s", b.chainID)
	return b.seekOldest(orderer.SeekInfo_BLOCK)
}

func (b *blocksRequester) RequestHeaders(height uint64) (*common.Envelope, error) {
	if height > 0 {
		logger.Infof("Starting deliver with block [%d] for channel %s", height, b.chainID)
		return b.seekLatestFromCommitter(height, orderer.SeekInfo_HEADER_WITH_SIG)
	}

	logger.Infof("Starting deliver with oldest block for channel %s", b.chainID)
	return b.seekOldest(orderer.SeekInfo_HEADER_WITH_SIG)
}

func (b *blocksRequester) getTLSCertHash() []byte {
	if b.deliverGPRCClient.MutualTLSRequired() {
		return util.ComputeSHA256(b.deliverGPRCClient.Certificate().Certificate[0])
	}
	return nil
}

func (b *blocksRequester) seekOldest(contentType orderer.SeekInfo_SeekContentType) (*common.Envelope, error) {
	seekInfo := &orderer.SeekInfo{
		Start:       &orderer.SeekPosition{Type: &orderer.SeekPosition_Oldest{Oldest: &orderer.SeekOldest{}}},
		Stop:        &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}},
		Behavior:    orderer.SeekInfo_BLOCK_UNTIL_READY,
		ContentType: contentType,
	}

	//TODO- epoch and msgVersion may need to be obtained for nowfollowing usage in orderer/configupdate/configupdate.go
	msgVersion := int32(0)
	epoch := uint64(0)
	tlsCertHash := b.getTLSCertHash()
	return protoutil.CreateSignedEnvelopeWithTLSBinding(common.HeaderType_DELIVER_SEEK_INFO, b.chainID, b.signer, seekInfo, msgVersion, epoch, tlsCertHash)
}

func (b *blocksRequester) seekLatestFromCommitter(height uint64, contentType orderer.SeekInfo_SeekContentType) (*common.Envelope, error) {
	seekInfo := &orderer.SeekInfo{
		Start:       &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: height}}},
		Stop:        &orderer.SeekPosition{Type: &orderer.SeekPosition_Specified{Specified: &orderer.SeekSpecified{Number: math.MaxUint64}}},
		Behavior:    orderer.SeekInfo_BLOCK_UNTIL_READY,
		ContentType: contentType,
	}

	//TODO- epoch and msgVersion may need to be obtained for nowfollowing usage in orderer/configupdate/configupdate.go
	msgVersion := int32(0)
	epoch := uint64(0)
	tlsCertHash := b.getTLSCertHash()
	return protoutil.CreateSignedEnvelopeWithTLSBinding(common.HeaderType_DELIVER_SEEK_INFO, b.chainID, b.signer, seekInfo, msgVersion, epoch, tlsCertHash)
}
