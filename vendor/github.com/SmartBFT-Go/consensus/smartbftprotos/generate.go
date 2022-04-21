package smartbftprotos

//go:generate protoc --go_out=plugins=grpc,paths=source_relative:. logrecord.proto
//go:generate protoc --go_out=plugins=grpc,paths=source_relative:. messages.proto
