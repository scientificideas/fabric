/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest_test

import (
	"bytes"
	"crypto/x509"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/metrics/metricsfakes"
	cb "github.com/hyperledger/fabric-protos-go-apiv2/common"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/chaincode/lifecycle"
	"github.com/hyperledger/fabric/core/endorser"
	"github.com/hyperledger/fabric/core/endorser/fake"
	"github.com/hyperledger/fabric/internal/peer/rest"
	"github.com/hyperledger/fabric/internal/peer/rest/mocks"
	"github.com/hyperledger/fabric/internal/peer/rest/pbrest"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
)

//go:generate counterfeiter -o mocks/acl_provider.go --fake-name ACLProvider . aclProvider
type aclProvider interface {
	aclmgmt.ACLProvider
}

func TestNewHTTPHandler(t *testing.T) {
	csccer := &mocks.InvokeNoShimer{}
	csccer.InvokeNoShimReturns(&pb.Response{
		Status:  http.StatusOK,
		Message: "OK",
		Payload: nil,
	})
	h := rest.NewRestAPIHandler(&endorser.Endorser{}, &mocks.ACLProvider{}, csccer)
	require.NotNilf(t, h, "cannot create handler")
}

func TestHTTPHandler_ServeHTTP_InvalidMethods(t *testing.T) {
	csccer := &mocks.InvokeNoShimer{}
	csccer.InvokeNoShimReturns(&pb.Response{
		Status:  http.StatusOK,
		Message: "OK",
		Payload: nil,
	})
	h := rest.NewRestAPIHandler(&endorser.Endorser{}, &mocks.ACLProvider{}, csccer)
	require.NotNilf(t, h, "cannot create handler")

	invalidMethods := []string{http.MethodConnect, http.MethodHead, http.MethodOptions, http.MethodPatch, http.MethodPut, http.MethodTrace}

	t.Run("on channel join", func(t *testing.T) {
		invalidMethodsExt := append(invalidMethods, http.MethodDelete, http.MethodGet)
		for _, method := range invalidMethodsExt {
			resp := httptest.NewRecorder()
			req := httptest.NewRequest(method, path.Join(rest.URLBaseV1, "channel/join"), nil)
			h.ServeHTTP(resp, req)
			checkErrorResponse(t, http.StatusNotImplemented, "Method Not Allowed", resp)
		}
	})
}

func TestHTTPHandler_ServeHTTP_Errors(t *testing.T) {
	csccer := &mocks.InvokeNoShimer{}
	csccer.InvokeNoShimReturns(&pb.Response{
		Status:  http.StatusOK,
		Message: "OK",
		Payload: nil,
	})
	h := rest.NewRestAPIHandler(&endorser.Endorser{}, &mocks.ACLProvider{}, csccer)
	require.NotNilf(t, h, "cannot create handler")

	t.Run("bad base", func(t *testing.T) {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/oops", nil)
		h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Result().StatusCode)
	})

	t.Run("bad resource", func(t *testing.T) {
		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, rest.URLBaseV1+"oops", nil)
		h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusNotFound, resp.Result().StatusCode)
	})
}

func TestHTTPHandler_ServeHTTP_Join(t *testing.T) {
	fakeLocalIdentity := &fake.Identity{}
	fakeLocalMSPIdentityDeserializer := &fake.IdentityDeserializer{}
	fakeLocalMSPIdentityDeserializer.DeserializeIdentityReturns(fakeLocalIdentity, nil)

	fakePrivateDataDistributor := &fake.PrivateDataDistributor{}

	fakeProposalDuration := &metricsfakes.Histogram{}
	fakeProposalDuration.WithReturns(fakeProposalDuration)
	fakeProposalsReceived := &metricsfakes.Counter{}
	fakeSuccessfulProposals := &metricsfakes.Counter{}
	fakeProposalValidationFailed := &metricsfakes.Counter{}
	fakeProposalACLCheckFailed := &metricsfakes.Counter{}
	fakeProposalACLCheckFailed.WithReturns(fakeProposalACLCheckFailed)
	fakeInitFailed := &metricsfakes.Counter{}
	fakeInitFailed.WithReturns(fakeInitFailed)
	fakeEndorsementsFailed := &metricsfakes.Counter{}
	fakeEndorsementsFailed.WithReturns(fakeEndorsementsFailed)
	fakeDuplicateTxsFailure := &metricsfakes.Counter{}
	fakeDuplicateTxsFailure.WithReturns(fakeDuplicateTxsFailure)
	fakeSimulateFailure := &metricsfakes.Counter{}
	fakeSimulateFailure.WithReturns(fakeSimulateFailure)

	fakeChannelIdentity := &fake.Identity{}
	fakeChannelMSPIdentityDeserializer := &fake.IdentityDeserializer{}
	fakeChannelMSPIdentityDeserializer.DeserializeIdentityReturns(fakeChannelIdentity, nil)
	fakeChannelFetcher := &fake.ChannelFetcher{}
	fakeChannelFetcher.ChannelReturns(&endorser.Channel{
		IdentityDeserializer: fakeChannelMSPIdentityDeserializer,
	})

	chaincodeResponse := &pb.Response{
		Status:  200,
		Payload: []byte("response-payload"),
	}
	chaincodeEvent := &pb.ChaincodeEvent{
		ChaincodeId: "chaincode-id",
		TxId:        "event-txid",
		EventName:   "event-name",
		Payload:     []byte("event-payload"),
	}
	fakeSupport := &fake.Support{}
	fakeSupport.ExecuteReturns(
		chaincodeResponse,
		chaincodeEvent,
		nil,
	)
	fakeSupport.ChaincodeEndorsementInfoReturns(&lifecycle.ChaincodeEndorsementInfo{
		Version:           "chaincode-definition-version",
		EndorsementPlugin: "plugin-name",
	}, nil)
	fakeSupport.GetLedgerHeightReturns(7, nil)
	fakeSupport.EndorseWithPluginReturns(
		&pb.Endorsement{
			Endorser:  []byte("endorser-identity"),
			Signature: []byte("endorser-signature"),
		},
		[]byte("endorser-modified-payload"),
		nil,
	)
	fakeSupport.SerializeReturns([]byte("signer"), nil)
	fakeSupport.SignReturns([]byte("signature"), nil)

	e := &endorser.Endorser{
		LocalMSP:               fakeLocalMSPIdentityDeserializer,
		PrivateDataDistributor: fakePrivateDataDistributor,
		Metrics: &endorser.Metrics{
			ProposalDuration:         fakeProposalDuration,
			ProposalsReceived:        fakeProposalsReceived,
			SuccessfulProposals:      fakeSuccessfulProposals,
			ProposalValidationFailed: fakeProposalValidationFailed,
			ProposalACLCheckFailed:   fakeProposalACLCheckFailed,
			InitFailed:               fakeInitFailed,
			EndorsementsFailed:       fakeEndorsementsFailed,
			DuplicateTxsFailure:      fakeDuplicateTxsFailure,
			SimulationFailure:        fakeSimulateFailure,
		},
		Support:        fakeSupport,
		ChannelFetcher: fakeChannelFetcher,
	}

	csccer := &mocks.InvokeNoShimer{}
	csccer.InvokeNoShimReturns(&pb.Response{
		Status:  http.StatusOK,
		Message: "OK",
		Payload: nil,
	})

	h := rest.NewRestAPIHandler(e, &mocks.ACLProvider{}, csccer)
	require.NotNilf(t, h, "cannot create handler")

	t.Run("join - ok", func(t *testing.T) {
		joinReq := &pbrest.JoinRequest{
			Block: validBlockBytes("ch-id"),
		}

		bytesReq, err := protojson.Marshal(joinReq)
		require.NoError(t, err, "cannot be marshaled")

		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://addr:80"+path.Join(rest.URLBaseV1, "channel/join"), bytes.NewReader(bytesReq))
		req.TLS.PeerCertificates = []*x509.Certificate{
			{Raw: []byte("cert")},
		}
		h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusOK, resp.Result().StatusCode)

		headerArray, headerOK := resp.Result().Header["Content-Type"]
		require.True(t, headerOK)
		require.Len(t, headerArray, 1)
		require.Equal(t, "application/json", headerArray[0])

		respBody, err := io.ReadAll(resp.Result().Body)
		require.NoError(t, err)
		require.NotNil(t, respBody)
	})

	t.Run("join - error", func(t *testing.T) {
		csccer.InvokeNoShimReturns(&pb.Response{
			Status:  http.StatusInternalServerError,
			Message: "some error",
		})
		fakeSupport.ExecuteReturns(
			nil,
			nil,
			errors.New("some error"),
		)

		joinReq := &pbrest.JoinRequest{
			Block: validBlockBytes("ch-id"),
		}

		bytesReq, err := protojson.Marshal(joinReq)
		require.NoError(t, err, "cannot be marshaled")

		resp := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "https://addr:80"+path.Join(rest.URLBaseV1, "channel/join"), bytes.NewReader(bytesReq))
		req.TLS.PeerCertificates = []*x509.Certificate{
			{Raw: []byte("cert")},
		}
		h.ServeHTTP(resp, req)
		require.Equal(t, http.StatusInternalServerError, resp.Result().StatusCode)

		headerArray, headerOK := resp.Result().Header["Content-Type"]
		require.True(t, headerOK)
		require.Len(t, headerArray, 1)
		require.Equal(t, "application/json", headerArray[0])

		respBody, err := io.ReadAll(resp.Result().Body)
		require.NoError(t, err)
		require.NotNil(t, respBody)
		var status spb.Status
		require.NoError(t, protojson.Unmarshal(respBody, &status))
		require.Equal(t, int32(codes.Unknown), status.GetCode())
		require.Contains(t, status.GetMessage(), "failed to invoke chaincode with status 500")
		require.Contains(t, status.GetMessage(), "some error")
	})
}

func checkErrorResponse(t *testing.T, expectedCode int, expectedErrMsg string, resp *httptest.ResponseRecorder) {
	require.Equal(t, expectedCode, resp.Result().StatusCode)

	headerArray, headerOK := resp.Result().Header["Content-Type"]
	require.True(t, headerOK)
	require.Len(t, headerArray, 1)
	require.Equal(t, "application/json", headerArray[0])

	respErr := &spb.Status{}
	err := protojson.Unmarshal(resp.Body.Bytes(), respErr)
	require.NoError(t, err, "body: %s", resp.Body.String())
	require.Contains(t, respErr.GetMessage(), expectedErrMsg)
}

func validBlockBytes(channelID string) []byte {
	blockBytes := protoutil.MarshalOrPanic(blockWithGroups(map[string]*cb.ConfigGroup{
		"Application": {},
	}, channelID))
	return blockBytes
}

func blockWithGroups(groups map[string]*cb.ConfigGroup, channelID string) *cb.Block {
	block := protoutil.NewBlock(0, []byte{})
	block.Data = &cb.BlockData{
		Data: [][]byte{
			protoutil.MarshalOrPanic(&cb.Envelope{
				Payload: protoutil.MarshalOrPanic(&cb.Payload{
					Data: protoutil.MarshalOrPanic(&cb.ConfigEnvelope{
						Config: &cb.Config{
							ChannelGroup: &cb.ConfigGroup{
								Groups: groups,
								Values: map[string]*cb.ConfigValue{
									"HashingAlgorithm": {
										Value: protoutil.MarshalOrPanic(&cb.HashingAlgorithm{
											Name: bccsp.SHA256,
										}),
									},
									"BlockDataHashingStructure": {
										Value: protoutil.MarshalOrPanic(&cb.BlockDataHashingStructure{
											Width: math.MaxUint32,
										}),
									},
									"OrdererAddresses": {
										Value: protoutil.MarshalOrPanic(&cb.OrdererAddresses{
											Addresses: []string{"localhost"},
										}),
									},
								},
							},
						},
					}),
					Header: &cb.Header{
						ChannelHeader: protoutil.MarshalOrPanic(&cb.ChannelHeader{
							Type:      int32(cb.HeaderType_CONFIG),
							ChannelId: channelID,
						}),
					},
				}),
			}),
		},
	}
	block.Header.DataHash = protoutil.ComputeBlockDataHash(block.Data)
	protoutil.InitBlockMetadata(block)

	return block
}
