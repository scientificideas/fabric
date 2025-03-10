/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest

import (
	"context"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/hyperledger/fabric/core/aclmgmt"
	"github.com/hyperledger/fabric/core/endorser"
	"github.com/hyperledger/fabric/internal/peer/rest/pbrest"
)

const URLBaseV1 = "/peer/v1/"

func NewRestAPIHandler(
	endorser *endorser.Endorser,
	aclProvider aclmgmt.ACLProvider,
	csccInst InvokeNoShimer,
) *runtime.ServeMux {
	ctx := context.Background()

	server := NewRestAPIServer(endorser, aclProvider, csccInst)
	mux := runtime.NewServeMux(runtime.WithWriteContentLength(), runtime.WithMiddlewares(middleware))
	_ = pbrest.RegisterAPIHandlerServer(ctx, mux, server)

	return mux
}
