// Code generated by protoc-gen-go. DO NOT EDIT.
// source: echo.proto

package testpb // import "github.com/hyperledger/fabric/common/grpcmetrics/testpb"

import (
	fmt "fmt"

	proto "github.com/golang/protobuf/proto"

	math "math"

	context "golang.org/x/net/context"

	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.ProtoPackageIsVersion2 // please upgrade the proto package

type Message struct {
	Message              string   `protobuf:"bytes,1,opt,name=message,proto3" json:"message,omitempty"`
	Sequence             int32    `protobuf:"varint,2,opt,name=sequence,proto3" json:"sequence,omitempty"`
	XXX_NoUnkeyedLiteral struct{} `json:"-"`
	XXX_unrecognized     []byte   `json:"-"`
	XXX_sizecache        int32    `json:"-"`
}

func (m *Message) Reset()         { *m = Message{} }
func (m *Message) String() string { return proto.CompactTextString(m) }
func (*Message) ProtoMessage()    {}
func (*Message) Descriptor() ([]byte, []int) {
	return fileDescriptor_echo_436b54e9f7a5436b, []int{0}
}
func (m *Message) XXX_Unmarshal(b []byte) error {
	return xxx_messageInfo_Message.Unmarshal(m, b)
}
func (m *Message) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	return xxx_messageInfo_Message.Marshal(b, m, deterministic)
}
func (dst *Message) XXX_Merge(src proto.Message) {
	xxx_messageInfo_Message.Merge(dst, src)
}
func (m *Message) XXX_Size() int {
	return xxx_messageInfo_Message.Size(m)
}
func (m *Message) XXX_DiscardUnknown() {
	xxx_messageInfo_Message.DiscardUnknown(m)
}

var xxx_messageInfo_Message proto.InternalMessageInfo

func (m *Message) GetMessage() string {
	if m != nil {
		return m.Message
	}
	return ""
}

func (m *Message) GetSequence() int32 {
	if m != nil {
		return m.Sequence
	}
	return 0
}

func init() {
	proto.RegisterType((*Message)(nil), "testpb.Message")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// EchoServiceClient is the client API for EchoService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type EchoServiceClient interface {
	Echo(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error)
	EchoStream(ctx context.Context, opts ...grpc.CallOption) (EchoService_EchoStreamClient, error)
}

type echoServiceClient struct {
	cc *grpc.ClientConn
}

func NewEchoServiceClient(cc *grpc.ClientConn) EchoServiceClient {
	return &echoServiceClient{cc}
}

func (c *echoServiceClient) Echo(ctx context.Context, in *Message, opts ...grpc.CallOption) (*Message, error) {
	out := new(Message)
	err := c.cc.Invoke(ctx, "/testpb.EchoService/Echo", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *echoServiceClient) EchoStream(ctx context.Context, opts ...grpc.CallOption) (EchoService_EchoStreamClient, error) {
	stream, err := c.cc.NewStream(ctx, &_EchoService_serviceDesc.Streams[0], "/testpb.EchoService/EchoStream", opts...)
	if err != nil {
		return nil, err
	}
	x := &echoServiceEchoStreamClient{stream}
	return x, nil
}

type EchoService_EchoStreamClient interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ClientStream
}

type echoServiceEchoStreamClient struct {
	grpc.ClientStream
}

func (x *echoServiceEchoStreamClient) Send(m *Message) error {
	return x.ClientStream.SendMsg(m)
}

func (x *echoServiceEchoStreamClient) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// EchoServiceServer is the server API for EchoService service.
type EchoServiceServer interface {
	Echo(context.Context, *Message) (*Message, error)
	EchoStream(EchoService_EchoStreamServer) error
}

func RegisterEchoServiceServer(s *grpc.Server, srv EchoServiceServer) {
	s.RegisterService(&_EchoService_serviceDesc, srv)
}

func _EchoService_Echo_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(Message)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(EchoServiceServer).Echo(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/testpb.EchoService/Echo",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(EchoServiceServer).Echo(ctx, req.(*Message))
	}
	return interceptor(ctx, in, info, handler)
}

func _EchoService_EchoStream_Handler(srv interface{}, stream grpc.ServerStream) error {
	return srv.(EchoServiceServer).EchoStream(&echoServiceEchoStreamServer{stream})
}

type EchoService_EchoStreamServer interface {
	Send(*Message) error
	Recv() (*Message, error)
	grpc.ServerStream
}

type echoServiceEchoStreamServer struct {
	grpc.ServerStream
}

func (x *echoServiceEchoStreamServer) Send(m *Message) error {
	return x.ServerStream.SendMsg(m)
}

func (x *echoServiceEchoStreamServer) Recv() (*Message, error) {
	m := new(Message)
	if err := x.ServerStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

var _EchoService_serviceDesc = grpc.ServiceDesc{
	ServiceName: "testpb.EchoService",
	HandlerType: (*EchoServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Echo",
			Handler:    _EchoService_Echo_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "EchoStream",
			Handler:       _EchoService_EchoStream_Handler,
			ServerStreams: true,
			ClientStreams: true,
		},
	},
	Metadata: "echo.proto",
}

func init() { proto.RegisterFile("echo.proto", fileDescriptor_echo_436b54e9f7a5436b) }

var fileDescriptor_echo_436b54e9f7a5436b = []byte{
	// 196 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x84, 0x8f, 0x31, 0x4b, 0xc0, 0x30,
	0x10, 0x85, 0x89, 0x68, 0xab, 0xe7, 0x20, 0x64, 0x2a, 0x9d, 0x8a, 0x53, 0xa6, 0x44, 0xea, 0x20,
	0x4e, 0x82, 0xe0, 0xe8, 0x52, 0x37, 0xb7, 0xe6, 0x3c, 0x93, 0xa0, 0x69, 0xe2, 0x25, 0x15, 0xfc,
	0xf7, 0x62, 0xab, 0x0e, 0x2e, 0x6e, 0xf7, 0x1d, 0x77, 0xdf, 0xe3, 0x01, 0x10, 0xfa, 0xa4, 0x33,
	0xa7, 0x9a, 0x64, 0x53, 0xa9, 0xd4, 0x6c, 0xcf, 0x6f, 0xa0, 0xbd, 0xa7, 0x52, 0x66, 0x47, 0xb2,
	0x83, 0x36, 0xee, 0x63, 0x27, 0x06, 0xa1, 0x4e, 0xa6, 0x1f, 0x94, 0x3d, 0x1c, 0x17, 0x7a, 0x5b,
	0x69, 0x41, 0xea, 0x0e, 0x06, 0xa1, 0x8e, 0xa6, 0x5f, 0x1e, 0x5f, 0xe0, 0xf4, 0x0e, 0x7d, 0x7a,
	0x20, 0x7e, 0x0f, 0x48, 0x52, 0xc1, 0xe1, 0x17, 0xca, 0x33, 0xbd, 0x07, 0xe8, 0x6f, 0x7b, 0xff,
	0x77, 0x21, 0x47, 0x80, 0xed, 0xb1, 0x32, 0xcd, 0xf1, 0xff, 0x7b, 0x25, 0x2e, 0xc4, 0xed, 0xf5,
	0xe3, 0x95, 0x0b, 0xd5, 0xaf, 0x56, 0x63, 0x8a, 0xc6, 0x7f, 0x64, 0xe2, 0x57, 0x7a, 0x72, 0xc4,
	0xe6, 0x79, 0xb6, 0x1c, 0xd0, 0x60, 0x8a, 0x31, 0x2d, 0xc6, 0x71, 0xc6, 0x48, 0x95, 0x03, 0x16,
	0xb3, 0x6b, 0x6c, 0xb3, 0xf5, 0xbe, 0xfc, 0x0c, 0x00, 0x00, 0xff, 0xff, 0x48, 0xde, 0xb3, 0x72,
	0x05, 0x01, 0x00, 0x00,
}
