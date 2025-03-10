/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package rest

import (
	"net"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
)

func middleware(next runtime.HandlerFunc) runtime.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request, pathParams map[string]string) {
		var localAddr net.Addr
		if la := r.Context().Value(http.LocalAddrContextKey); la != nil {
			localAddr, _ = la.(net.Addr)
		}

		var authInfo credentials.AuthInfo
		if r.TLS != nil {
			authInfo = credentials.TLSInfo{
				State:          *r.TLS,
				CommonAuthInfo: credentials.CommonAuthInfo{SecurityLevel: credentials.PrivacyAndIntegrity},
			}
		}

		ctx := peer.NewContext(r.Context(), &peer.Peer{
			Addr:      strAddr(r.RemoteAddr),
			LocalAddr: localAddr,
			AuthInfo:  authInfo,
		})
		r = r.WithContext(ctx)

		next(w, r, pathParams)
	}
}

type strAddr string

func (a strAddr) Network() string {
	if a != "" {
		return "tcp"
	}
	return ""
}
func (a strAddr) String() string { return string(a) }
