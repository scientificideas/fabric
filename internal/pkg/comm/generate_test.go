/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

//go:generate protoc --proto_path=testdata/grpc --go_out=testpb --go_opt=paths=source_relative --go-grpc_out=require_unimplemented_servers=false:testpb --go-grpc_opt=paths=source_relative test.proto

package comm_test
