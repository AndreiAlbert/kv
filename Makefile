.PHONY: proto

build: build_server build_client

build_server: proto
	go build -o bin/server cmd/server/main.go

build_client: proto
	go build -o bin/client client/client.go

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/kvstore.proto

clean: 
	rm -rf bin
	rm -rf data
	rm proto/*.go

deps:
	go install google.golang.protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
