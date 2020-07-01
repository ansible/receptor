receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build cmd/receptor.go

lint:
	golint cmd/... pkg/... example/...

format:
	find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

ci:
	echo "Running pre-commit" && \
	pre-commit run --all-files && \
	echo "Running Go builds" && \
	go build cmd/*.go && \
	go build example/*.go && \
	echo "All done"

.PHONY: lint format fmt ci
