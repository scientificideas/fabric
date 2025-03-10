/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package nwo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"

	"github.com/hyperledger/fabric-protos-go-apiv2/common"
	"github.com/hyperledger/fabric/internal/peer/rest"
	"github.com/hyperledger/fabric/internal/peer/rest/pbrest"
	. "github.com/onsi/gomega"
	spb "google.golang.org/genproto/googleapis/rpc/status"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func JoinChannelPeers(n *Network, blockBytes []byte, expectedError string, peers ...*Peer) {
	for _, p := range peers {
		JoinChannel(n, p, blockBytes, expectedError)
	}
}

func JoinChannelPeersWithBlock(n *Network, block *common.Block, expectedError string, peers ...*Peer) {
	blockBytes, err := proto.Marshal(block)
	Expect(err).NotTo(HaveOccurred())
	for _, p := range peers {
		JoinChannel(n, p, blockBytes, expectedError)
	}
}

func JoinChannelWithBlock(n *Network, p *Peer, block *common.Block, expectedError string) {
	blockBytes, err := proto.Marshal(block)
	Expect(err).NotTo(HaveOccurred())

	JoinChannel(n, p, blockBytes, expectedError)
}

func JoinChannel(n *Network, p *Peer, blockBytes []byte, expectedError string) {
	protocol := "http"
	if n.TLSEnabled {
		protocol = "https"
	}
	url := fmt.Sprintf("%s://127.0.0.1:%d%schannel/join", protocol, n.PeerPort(p, AdminPort), rest.URLBaseV1)
	req := generateJoinChannelRequest(url, blockBytes)
	authClient, unauthClient := PeerOperationalClients(n, p)

	client := unauthClient
	if n.TLSEnabled {
		client = authClient
	}

	status, body := doBodyPeerRest(client, req)

	if expectedError == "" {
		Expect(status).To(Equal(http.StatusOK), string(body))
		Expect(string(body)).ToNot(BeNil())
		return
	}

	Expect(status).To(Equal(http.StatusInternalServerError), string(body))

	var st spb.Status
	err := protojson.Unmarshal(body, &st)
	Expect(err).NotTo(HaveOccurred())
	Expect(st.GetCode()).To(Equal(int32(codes.Unknown)), string(body))
	Expect(st.GetMessage()).To(ContainSubstring(expectedError))
}

func generateJoinChannelRequest(url string, blockBytes []byte) *http.Request {
	joinReq := &pbrest.JoinRequest{
		Block: blockBytes,
	}
	bytesReq, err := protojson.Marshal(joinReq)
	Expect(err).NotTo(HaveOccurred())

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(bytesReq))
	Expect(err).NotTo(HaveOccurred())
	req.Header.Set("Content-Type", "application/json")

	return req
}

func doBodyPeerRest(client *http.Client, req *http.Request) (status int, body []byte) {
	resp, err := client.Do(req)
	Expect(err).NotTo(HaveOccurred())

	bodyBytes, err := io.ReadAll(resp.Body)
	Expect(err).NotTo(HaveOccurred())
	resp.Body.Close()

	return resp.StatusCode, bodyBytes
}
