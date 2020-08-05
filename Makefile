receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build cmd/receptor.go

lint:
	golint cmd/... pkg/... example/...

format:
	find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

pre-commit:
	echo "Running pre-commit" && \
	pre-commit run --all-files

build-all:
	echo "Running Go builds" && \
	go build cmd/*.go && \
	GOOS=windows go build -o receptor.exe cmd/receptor.go && \
	GOOS=darwin go build -o receptor.app cmd/receptor.go && \
	go build example/*.go

test:
	go test ./... -p 1 -count=1

ci: pre-commit build-all test
	echo "All done"

clean:
	rm receptor receptor.exe receptor.app

.PHONY: lint format fmt ci pre-commit build-all test clean
