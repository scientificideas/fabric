// Copyright the Hyperledger Fabric contributors. All rights reserved.
//
// SPDX-License-Identifier: Apache-2.0

// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.33.0
// 	protoc        (unknown)
// source: peer/lifecycle/chaincode_definition.proto

package lifecycle

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

// ChaincodeEndorsementInfo is (most) everything the peer needs to know in order
// to execute a chaincode
type ChaincodeEndorsementInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Version           string `protobuf:"bytes,1,opt,name=version,proto3" json:"version,omitempty"`
	InitRequired      bool   `protobuf:"varint,2,opt,name=init_required,json=initRequired,proto3" json:"init_required,omitempty"`
	EndorsementPlugin string `protobuf:"bytes,3,opt,name=endorsement_plugin,json=endorsementPlugin,proto3" json:"endorsement_plugin,omitempty"`
}

func (x *ChaincodeEndorsementInfo) Reset() {
	*x = ChaincodeEndorsementInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_peer_lifecycle_chaincode_definition_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChaincodeEndorsementInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChaincodeEndorsementInfo) ProtoMessage() {}

func (x *ChaincodeEndorsementInfo) ProtoReflect() protoreflect.Message {
	mi := &file_peer_lifecycle_chaincode_definition_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChaincodeEndorsementInfo.ProtoReflect.Descriptor instead.
func (*ChaincodeEndorsementInfo) Descriptor() ([]byte, []int) {
	return file_peer_lifecycle_chaincode_definition_proto_rawDescGZIP(), []int{0}
}

func (x *ChaincodeEndorsementInfo) GetVersion() string {
	if x != nil {
		return x.Version
	}
	return ""
}

func (x *ChaincodeEndorsementInfo) GetInitRequired() bool {
	if x != nil {
		return x.InitRequired
	}
	return false
}

func (x *ChaincodeEndorsementInfo) GetEndorsementPlugin() string {
	if x != nil {
		return x.EndorsementPlugin
	}
	return ""
}

// ValidationInfo is (most) everything the peer needs to know in order
// to validate a transaction
type ChaincodeValidationInfo struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ValidationPlugin    string `protobuf:"bytes,1,opt,name=validation_plugin,json=validationPlugin,proto3" json:"validation_plugin,omitempty"`
	ValidationParameter []byte `protobuf:"bytes,2,opt,name=validation_parameter,json=validationParameter,proto3" json:"validation_parameter,omitempty"`
}

func (x *ChaincodeValidationInfo) Reset() {
	*x = ChaincodeValidationInfo{}
	if protoimpl.UnsafeEnabled {
		mi := &file_peer_lifecycle_chaincode_definition_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *ChaincodeValidationInfo) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*ChaincodeValidationInfo) ProtoMessage() {}

func (x *ChaincodeValidationInfo) ProtoReflect() protoreflect.Message {
	mi := &file_peer_lifecycle_chaincode_definition_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use ChaincodeValidationInfo.ProtoReflect.Descriptor instead.
func (*ChaincodeValidationInfo) Descriptor() ([]byte, []int) {
	return file_peer_lifecycle_chaincode_definition_proto_rawDescGZIP(), []int{1}
}

func (x *ChaincodeValidationInfo) GetValidationPlugin() string {
	if x != nil {
		return x.ValidationPlugin
	}
	return ""
}

func (x *ChaincodeValidationInfo) GetValidationParameter() []byte {
	if x != nil {
		return x.ValidationParameter
	}
	return nil
}

var File_peer_lifecycle_chaincode_definition_proto protoreflect.FileDescriptor

var file_peer_lifecycle_chaincode_definition_proto_rawDesc = []byte{
	0x0a, 0x29, 0x70, 0x65, 0x65, 0x72, 0x2f, 0x6c, 0x69, 0x66, 0x65, 0x63, 0x79, 0x63, 0x6c, 0x65,
	0x2f, 0x63, 0x68, 0x61, 0x69, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x5f, 0x64, 0x65, 0x66, 0x69, 0x6e,
	0x69, 0x74, 0x69, 0x6f, 0x6e, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x12, 0x09, 0x6c, 0x69, 0x66,
	0x65, 0x63, 0x79, 0x63, 0x6c, 0x65, 0x22, 0x88, 0x01, 0x0a, 0x18, 0x43, 0x68, 0x61, 0x69, 0x6e,
	0x63, 0x6f, 0x64, 0x65, 0x45, 0x6e, 0x64, 0x6f, 0x72, 0x73, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x49,
	0x6e, 0x66, 0x6f, 0x12, 0x18, 0x0a, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x18, 0x01,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e, 0x12, 0x23, 0x0a,
	0x0d, 0x69, 0x6e, 0x69, 0x74, 0x5f, 0x72, 0x65, 0x71, 0x75, 0x69, 0x72, 0x65, 0x64, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x08, 0x52, 0x0c, 0x69, 0x6e, 0x69, 0x74, 0x52, 0x65, 0x71, 0x75, 0x69, 0x72,
	0x65, 0x64, 0x12, 0x2d, 0x0a, 0x12, 0x65, 0x6e, 0x64, 0x6f, 0x72, 0x73, 0x65, 0x6d, 0x65, 0x6e,
	0x74, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x18, 0x03, 0x20, 0x01, 0x28, 0x09, 0x52, 0x11,
	0x65, 0x6e, 0x64, 0x6f, 0x72, 0x73, 0x65, 0x6d, 0x65, 0x6e, 0x74, 0x50, 0x6c, 0x75, 0x67, 0x69,
	0x6e, 0x22, 0x79, 0x0a, 0x17, 0x43, 0x68, 0x61, 0x69, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x56, 0x61,
	0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x49, 0x6e, 0x66, 0x6f, 0x12, 0x2b, 0x0a, 0x11,
	0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x70, 0x6c, 0x75, 0x67, 0x69,
	0x6e, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x10, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x50, 0x6c, 0x75, 0x67, 0x69, 0x6e, 0x12, 0x31, 0x0a, 0x14, 0x76, 0x61, 0x6c,
	0x69, 0x64, 0x61, 0x74, 0x69, 0x6f, 0x6e, 0x5f, 0x70, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65,
	0x72, 0x18, 0x02, 0x20, 0x01, 0x28, 0x0c, 0x52, 0x13, 0x76, 0x61, 0x6c, 0x69, 0x64, 0x61, 0x74,
	0x69, 0x6f, 0x6e, 0x50, 0x61, 0x72, 0x61, 0x6d, 0x65, 0x74, 0x65, 0x72, 0x42, 0xca, 0x01, 0x0a,
	0x2c, 0x6f, 0x72, 0x67, 0x2e, 0x68, 0x79, 0x70, 0x65, 0x72, 0x6c, 0x65, 0x64, 0x67, 0x65, 0x72,
	0x2e, 0x66, 0x61, 0x62, 0x72, 0x69, 0x63, 0x2e, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2e, 0x70,
	0x65, 0x65, 0x72, 0x2e, 0x6c, 0x69, 0x66, 0x65, 0x63, 0x79, 0x63, 0x6c, 0x65, 0x42, 0x18, 0x43,
	0x68, 0x61, 0x69, 0x6e, 0x63, 0x6f, 0x64, 0x65, 0x44, 0x65, 0x66, 0x69, 0x6e, 0x69, 0x74, 0x69,
	0x6f, 0x6e, 0x50, 0x72, 0x6f, 0x74, 0x6f, 0x50, 0x01, 0x5a, 0x3c, 0x67, 0x69, 0x74, 0x68, 0x75,
	0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x68, 0x79, 0x70, 0x65, 0x72, 0x6c, 0x65, 0x64, 0x67, 0x65,
	0x72, 0x2f, 0x66, 0x61, 0x62, 0x72, 0x69, 0x63, 0x2d, 0x70, 0x72, 0x6f, 0x74, 0x6f, 0x73, 0x2d,
	0x67, 0x6f, 0x2d, 0x61, 0x70, 0x69, 0x76, 0x32, 0x2f, 0x70, 0x65, 0x65, 0x72, 0x2f, 0x6c, 0x69,
	0x66, 0x65, 0x63, 0x79, 0x63, 0x6c, 0x65, 0xa2, 0x02, 0x03, 0x4c, 0x58, 0x58, 0xaa, 0x02, 0x09,
	0x4c, 0x69, 0x66, 0x65, 0x63, 0x79, 0x63, 0x6c, 0x65, 0xca, 0x02, 0x09, 0x4c, 0x69, 0x66, 0x65,
	0x63, 0x79, 0x63, 0x6c, 0x65, 0xe2, 0x02, 0x15, 0x4c, 0x69, 0x66, 0x65, 0x63, 0x79, 0x63, 0x6c,
	0x65, 0x5c, 0x47, 0x50, 0x42, 0x4d, 0x65, 0x74, 0x61, 0x64, 0x61, 0x74, 0x61, 0xea, 0x02, 0x09,
	0x4c, 0x69, 0x66, 0x65, 0x63, 0x79, 0x63, 0x6c, 0x65, 0x62, 0x06, 0x70, 0x72, 0x6f, 0x74, 0x6f,
	0x33,
}

var (
	file_peer_lifecycle_chaincode_definition_proto_rawDescOnce sync.Once
	file_peer_lifecycle_chaincode_definition_proto_rawDescData = file_peer_lifecycle_chaincode_definition_proto_rawDesc
)

func file_peer_lifecycle_chaincode_definition_proto_rawDescGZIP() []byte {
	file_peer_lifecycle_chaincode_definition_proto_rawDescOnce.Do(func() {
		file_peer_lifecycle_chaincode_definition_proto_rawDescData = protoimpl.X.CompressGZIP(file_peer_lifecycle_chaincode_definition_proto_rawDescData)
	})
	return file_peer_lifecycle_chaincode_definition_proto_rawDescData
}

var file_peer_lifecycle_chaincode_definition_proto_msgTypes = make([]protoimpl.MessageInfo, 2)
var file_peer_lifecycle_chaincode_definition_proto_goTypes = []interface{}{
	(*ChaincodeEndorsementInfo)(nil), // 0: lifecycle.ChaincodeEndorsementInfo
	(*ChaincodeValidationInfo)(nil),  // 1: lifecycle.ChaincodeValidationInfo
}
var file_peer_lifecycle_chaincode_definition_proto_depIdxs = []int32{
	0, // [0:0] is the sub-list for method output_type
	0, // [0:0] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_peer_lifecycle_chaincode_definition_proto_init() }
func file_peer_lifecycle_chaincode_definition_proto_init() {
	if File_peer_lifecycle_chaincode_definition_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_peer_lifecycle_chaincode_definition_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChaincodeEndorsementInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_peer_lifecycle_chaincode_definition_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*ChaincodeValidationInfo); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_peer_lifecycle_chaincode_definition_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   2,
			NumExtensions: 0,
			NumServices:   0,
		},
		GoTypes:           file_peer_lifecycle_chaincode_definition_proto_goTypes,
		DependencyIndexes: file_peer_lifecycle_chaincode_definition_proto_depIdxs,
		MessageInfos:      file_peer_lifecycle_chaincode_definition_proto_msgTypes,
	}.Build()
	File_peer_lifecycle_chaincode_definition_proto = out.File
	file_peer_lifecycle_chaincode_definition_proto_rawDesc = nil
	file_peer_lifecycle_chaincode_definition_proto_goTypes = nil
	file_peer_lifecycle_chaincode_definition_proto_depIdxs = nil
}
