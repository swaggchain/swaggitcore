.PHONY: compile

net.pb.go: services/network/pb/messages.proto
	protoc --go_out=services/network/pb/ services/network/pb/messages.proto

compile: net.pb.go