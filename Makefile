.PHONY: protoc
protoc:
	protoc -I internal/proto \
		--go_out internal/proto \
		--go_opt paths=source_relative \
		--go-grpc_out internal/proto \
		--go-grpc_opt paths=source_relative \
		internal/proto/test.proto

.PHONY: install
install: plugin.so
	@cp plugin.so ${GOPATH}/bin/proto-golint-plugin.so
	@ls ${GOPATH}/bin/proto-golint-plugin.so
	go install ./cmd/proto-golint
	@echo "installed in $(shell which proto-golint)"

plugin.so:
	CGO_ENABLED=1 go build -buildmode=plugin ./plugin/proto-golint/plugin.go

.PHONY: clean
clean:
	-rm plugin.so

.PHONY: run
run:
	go run ./cmd/proto-golint ./internal/test

.PHONY: fix
fix:
	go run ./cmd/proto-golint --fix ./internal/test

.PHONY: lint
lint:
	golangci-lint run ./... 

.PHONY: install-golangci-lint
install-golangci-lint:
	CGO_ENABLED=1 go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.53.0
