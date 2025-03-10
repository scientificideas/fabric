/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest

import (
	"context"
	"fmt"

	"github.com/hyperledger/fabric-lib-go/common/flogging"
	pb "github.com/hyperledger/fabric-protos-go-apiv2/peer"
	"github.com/hyperledger/fabric/common/util"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/aclmgmt/resources"
	"github.com/hyperledger/fabric/core/endorser"
	"github.com/hyperledger/fabric/core/scc/cscc"
	"github.com/hyperledger/fabric/internal/peer/rest/pbrest"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//go:generate counterfeiter -o mocks/invoke_no_shimer.go --fake-name InvokeNoShimer . InvokeNoShimer
type InvokeNoShimer interface {
	InvokeNoShim(args [][]byte, sp *pb.SignedProposal, checkACL bool) *pb.Response
}

type APIServer struct {
	pbrest.UnimplementedAPIServer

	serverEndorser *endorser.Endorser
	aclProvider    aclmgmt.ACLProvider
	csccInst       InvokeNoShimer
	logger         *flogging.FabricLogger
}

func NewRestAPIServer(
	serverEndorser *endorser.Endorser,
	aclProvider aclmgmt.ACLProvider,
	csccInst InvokeNoShimer,
) *APIServer {
	return &APIServer{
		serverEndorser: serverEndorser,
		aclProvider:    aclProvider,
		csccInst:       csccInst,
		logger:         flogging.MustGetLogger("peer.rest.server"),
	}
}

func (s *APIServer) Join(ctx context.Context, req *pbrest.JoinRequest) (*pbrest.JoinResponse, error) {
	clientCert := util.ExtractCertificateFromContext(ctx)
	if clientCert == nil {
		return nil, fmt.Errorf("no client certificate provided")
	}

	if len(req.GetBlock()) == 0 {
		return nil, status.Error(codes.InvalidArgument, "invalid block")
	}

	block, err := protoutil.UnmarshalBlock(req.GetBlock())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("failed to reconstruct the genesis block, %s", err))
	}

	cid, err := protoutil.GetChannelIDFromBlock(block)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("join chain request failed to extract "+
			"channel id from the block due to [%s]", err))
	}

	if err = s.aclProvider.CheckACL(resources.Cscc_JoinChain, "", clientCert); err != nil {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("access denied for [%s][%s]: [%s]", cscc.JoinChain, cid, err))
	}

	pResp := s.csccInst.InvokeNoShim([][]byte{[]byte(cscc.JoinChain), req.GetBlock()}, nil, false)

	if pResp.GetStatus() != 200 {
		s.logger.Warnw("failed to invoke chaincode", "status", pResp.GetStatus())
		return nil, errors.Errorf("failed to invoke chaincode with status %d: %s",
			pResp.GetStatus(),
			pResp.GetMessage(),
		)
	}

	return &pbrest.JoinResponse{}, nil
}
