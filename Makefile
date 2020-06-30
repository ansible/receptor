receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build cmd/receptor.go

lint:
	golint cmd/... pkg/... example/...

format:
	find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

.PHONY: lint format fmt
