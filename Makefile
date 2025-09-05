.PHONY: proto

build: build_server build_client

build_server: proto
	go build -o bin/server server/server.go

build_client: proto
	go build -o bin/client client/client.go

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/kvstore.proto

clean: 
	rm -rf bin
	rm proto/*.go

deps:
	go install google.golang.protobuf/cmd/protoc-gen-go@v1.28
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@v1.2
