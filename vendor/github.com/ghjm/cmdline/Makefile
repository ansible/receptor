test:
	@go test -count=1 .

lint:
	@golint ./...

format:
	@find . -type f -name '*.go' -exec go fmt {} \;

fmt: format

pre-commit:
	@pre-commit run --all-files

example: cmd/example.go cmdline.go
	@go build cmd/example.go

clean:
	@rm -f example

