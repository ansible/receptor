receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build cmd/receptor.go

lint:
	golint cmd/... pkg/...
