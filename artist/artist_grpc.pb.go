// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.20.1
// source: artist.proto

package artist

import (
	context "context"

	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// ArtistServiceClient is the client API for ArtistService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type ArtistServiceClient interface {
	SyncArtist(ctx context.Context, in *SyncArtistRequest, opts ...grpc.CallOption) (*SyncArtistResponse, error)
	ReadArtistAlbums(ctx context.Context, in *ReadArtistAlbumRequest, opts ...grpc.CallOption) (*ReadArtistAlbumResponse, error)
	ReadNewAlbums(ctx context.Context, in *ListArtistRequest, opts ...grpc.CallOption) (*ReadArtistAlbumResponse, error)
	SyncAlbum(ctx context.Context, in *SyncAlbumRequest, opts ...grpc.CallOption) (*SyncAlbumResponse, error)
	ReadAlbumTracks(ctx context.Context, in *ReadAlbumTrackRequest, opts ...grpc.CallOption) (*ReadAlbumTrackResponse, error)
	DeleteArtist(ctx context.Context, in *DeleteArtistRequest, opts ...grpc.CallOption) (*DeleteArtistResponse, error)
	DownloadAlbums(ctx context.Context, in *DownloadAlbumsRequest, opts ...grpc.CallOption) (*DownloadAlbumsResponse, error)
	DownloadTracks(ctx context.Context, in *DownloadTracksRequest, opts ...grpc.CallOption) (*DownloadTracksResponse, error)
	ListArtist(ctx context.Context, in *ListArtistRequest, opts ...grpc.CallOption) (*ListArtistResponse, error)
	ListStreamArtist(ctx context.Context, in *ListStreamArtistRequest, opts ...grpc.CallOption) (ArtistService_ListStreamArtistClient, error)
}

type artistServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewArtistServiceClient(cc grpc.ClientConnInterface) ArtistServiceClient {
	return &artistServiceClient{cc}
}

func (c *artistServiceClient) SyncArtist(ctx context.Context, in *SyncArtistRequest, opts ...grpc.CallOption) (*SyncArtistResponse, error) {
	out := new(SyncArtistResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/SyncArtist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) ReadArtistAlbums(ctx context.Context, in *ReadArtistAlbumRequest, opts ...grpc.CallOption) (*ReadArtistAlbumResponse, error) {
	out := new(ReadArtistAlbumResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/ReadArtistAlbums", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) ReadNewAlbums(ctx context.Context, in *ListArtistRequest, opts ...grpc.CallOption) (*ReadArtistAlbumResponse, error) {
	out := new(ReadArtistAlbumResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/ReadNewAlbums", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) SyncAlbum(ctx context.Context, in *SyncAlbumRequest, opts ...grpc.CallOption) (*SyncAlbumResponse, error) {
	out := new(SyncAlbumResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/SyncAlbum", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) ReadAlbumTracks(ctx context.Context, in *ReadAlbumTrackRequest, opts ...grpc.CallOption) (*ReadAlbumTrackResponse, error) {
	out := new(ReadAlbumTrackResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/ReadAlbumTracks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) DeleteArtist(ctx context.Context, in *DeleteArtistRequest, opts ...grpc.CallOption) (*DeleteArtistResponse, error) {
	out := new(DeleteArtistResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/DeleteArtist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) DownloadAlbums(ctx context.Context, in *DownloadAlbumsRequest, opts ...grpc.CallOption) (*DownloadAlbumsResponse, error) {
	out := new(DownloadAlbumsResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/DownloadAlbums", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) DownloadTracks(ctx context.Context, in *DownloadTracksRequest, opts ...grpc.CallOption) (*DownloadTracksResponse, error) {
	out := new(DownloadTracksResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/DownloadTracks", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) ListArtist(ctx context.Context, in *ListArtistRequest, opts ...grpc.CallOption) (*ListArtistResponse, error) {
	out := new(ListArtistResponse)
	err := c.cc.Invoke(ctx, "/artist.ArtistService/ListArtist", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *artistServiceClient) ListStreamArtist(ctx context.Context, in *ListStreamArtistRequest, opts ...grpc.CallOption) (ArtistService_ListStreamArtistClient, error) {
	stream, err := c.cc.NewStream(ctx, &ArtistService_ServiceDesc.Streams[0], "/artist.ArtistService/ListStreamArtist", opts...)
	if err != nil {
		return nil, err
	}
	x := &artistServiceListStreamArtistClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type ArtistService_ListStreamArtistClient interface {
	Recv() (*ListStreamArtistResponse, error)
	grpc.ClientStream
}

type artistServiceListStreamArtistClient struct {
	grpc.ClientStream
}

func (x *artistServiceListStreamArtistClient) Recv() (*ListStreamArtistResponse, error) {
	m := new(ListStreamArtistResponse)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// ArtistServiceServer is the server API for ArtistService service.
// All implementations must embed UnimplementedArtistServiceServer
// for forward compatibility
type ArtistServiceServer interface {
	SyncArtist(context.Context, *SyncArtistRequest) (*SyncArtistResponse, error)
	ReadArtistAlbums(context.Context, *ReadArtistAlbumRequest) (*ReadArtistAlbumResponse, error)
	ReadNewAlbums(context.Context, *ListArtistRequest) (*ReadArtistAlbumResponse, error)
	SyncAlbum(context.Context, *SyncAlbumRequest) (*SyncAlbumResponse, error)
	ReadAlbumTracks(context.Context, *ReadAlbumTrackRequest) (*ReadAlbumTrackResponse, error)
	DeleteArtist(context.Context, *DeleteArtistRequest) (*DeleteArtistResponse, error)
	DownloadAlbums(context.Context, *DownloadAlbumsRequest) (*DownloadAlbumsResponse, error)
	DownloadTracks(context.Context, *DownloadTracksRequest) (*DownloadTracksResponse, error)
	ListArtist(context.Context, *ListArtistRequest) (*ListArtistResponse, error)
	ListStreamArtist(*ListStreamArtistRequest, ArtistService_ListStreamArtistServer) error
	mustEmbedUnimplementedArtistServiceServer()
}

// UnimplementedArtistServiceServer must be embedded to have forward compatible implementations.
type UnimplementedArtistServiceServer struct {
}

func (UnimplementedArtistServiceServer) SyncArtist(context.Context, *SyncArtistRequest) (*SyncArtistResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SyncArtist not implemented")
}
func (UnimplementedArtistServiceServer) ReadArtistAlbums(context.Context, *ReadArtistAlbumRequest) (*ReadArtistAlbumResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadArtistAlbums not implemented")
}
func (UnimplementedArtistServiceServer) ReadNewAlbums(context.Context, *ListArtistRequest) (*ReadArtistAlbumResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadNewAlbums not implemented")
}
func (UnimplementedArtistServiceServer) SyncAlbum(context.Context, *SyncAlbumRequest) (*SyncAlbumResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SyncAlbum not implemented")
}
func (UnimplementedArtistServiceServer) ReadAlbumTracks(context.Context, *ReadAlbumTrackRequest) (*ReadAlbumTrackResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReadAlbumTracks not implemented")
}
func (UnimplementedArtistServiceServer) DeleteArtist(context.Context, *DeleteArtistRequest) (*DeleteArtistResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DeleteArtist not implemented")
}
func (UnimplementedArtistServiceServer) DownloadAlbums(context.Context, *DownloadAlbumsRequest) (*DownloadAlbumsResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadAlbums not implemented")
}
func (UnimplementedArtistServiceServer) DownloadTracks(context.Context, *DownloadTracksRequest) (*DownloadTracksResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method DownloadTracks not implemented")
}
func (UnimplementedArtistServiceServer) ListArtist(context.Context, *ListArtistRequest) (*ListArtistResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ListArtist not implemented")
}
func (UnimplementedArtistServiceServer) ListStreamArtist(*ListStreamArtistRequest, ArtistService_ListStreamArtistServer) error {
	return status.Errorf(codes.Unimplemented, "method ListStreamArtist not implemented")
}
func (UnimplementedArtistServiceServer) mustEmbedUnimplementedArtistServiceServer() {}

// UnsafeArtistServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to ArtistServiceServer will
// result in compilation errors.
type UnsafeArtistServiceServer interface {
	mustEmbedUnimplementedArtistServiceServer()
}

func RegisterArtistServiceServer(s grpc.ServiceRegistrar, srv ArtistServiceServer) {
	s.RegisterService(&ArtistService_ServiceDesc, srv)
}

func _ArtistService_SyncArtist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncArtistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).SyncArtist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/SyncArtist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).SyncArtist(ctx, req.(*SyncArtistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_ReadArtistAlbums_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadArtistAlbumRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).ReadArtistAlbums(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/ReadArtistAlbums",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).ReadArtistAlbums(ctx, req.(*ReadArtistAlbumRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_ReadNewAlbums_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListArtistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).ReadNewAlbums(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/ReadNewAlbums",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).ReadNewAlbums(ctx, req.(*ListArtistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_SyncAlbum_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SyncAlbumRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).SyncAlbum(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/SyncAlbum",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).SyncAlbum(ctx, req.(*SyncAlbumRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_ReadAlbumTracks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReadAlbumTrackRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).ReadAlbumTracks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/ReadAlbumTracks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).ReadAlbumTracks(ctx, req.(*ReadAlbumTrackRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_DeleteArtist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DeleteArtistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).DeleteArtist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/DeleteArtist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).DeleteArtist(ctx, req.(*DeleteArtistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_DownloadAlbums_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DownloadAlbumsRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).DownloadAlbums(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/DownloadAlbums",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).DownloadAlbums(ctx, req.(*DownloadAlbumsRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_DownloadTracks_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(DownloadTracksRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).DownloadTracks(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/DownloadTracks",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).DownloadTracks(ctx, req.(*DownloadTracksRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_ListArtist_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ListArtistRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ArtistServiceServer).ListArtist(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/artist.ArtistService/ListArtist",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(ArtistServiceServer).ListArtist(ctx, req.(*ListArtistRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ArtistService_ListStreamArtist_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(ListStreamArtistRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ArtistServiceServer).ListStreamArtist(m, &artistServiceListStreamArtistServer{stream})
}

type ArtistService_ListStreamArtistServer interface {
	Send(*ListStreamArtistResponse) error
	grpc.ServerStream
}

type artistServiceListStreamArtistServer struct {
	grpc.ServerStream
}

func (x *artistServiceListStreamArtistServer) Send(m *ListStreamArtistResponse) error {
	return x.ServerStream.SendMsg(m)
}

// ArtistService_ServiceDesc is the grpc.ServiceDesc for ArtistService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var ArtistService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "artist.ArtistService",
	HandlerType: (*ArtistServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SyncArtist",
			Handler:    _ArtistService_SyncArtist_Handler,
		},
		{
			MethodName: "ReadArtistAlbums",
			Handler:    _ArtistService_ReadArtistAlbums_Handler,
		},
		{
			MethodName: "ReadNewAlbums",
			Handler:    _ArtistService_ReadNewAlbums_Handler,
		},
		{
			MethodName: "SyncAlbum",
			Handler:    _ArtistService_SyncAlbum_Handler,
		},
		{
			MethodName: "ReadAlbumTracks",
			Handler:    _ArtistService_ReadAlbumTracks_Handler,
		},
		{
			MethodName: "DeleteArtist",
			Handler:    _ArtistService_DeleteArtist_Handler,
		},
		{
			MethodName: "DownloadAlbums",
			Handler:    _ArtistService_DownloadAlbums_Handler,
		},
		{
			MethodName: "DownloadTracks",
			Handler:    _ArtistService_DownloadTracks_Handler,
		},
		{
			MethodName: "ListArtist",
			Handler:    _ArtistService_ListArtist_Handler,
		},
	},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "ListStreamArtist",
			Handler:       _ArtistService_ListStreamArtist_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "artist.proto",
}
