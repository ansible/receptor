receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build cmd/receptor.go

lint:
	@golint cmd/... pkg/... example/...

format:
	@find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

pre-commit:
	@pre-commit run --all-files

build-all:
	@echo "Running Go builds..." && go build cmd/*.go && \
	GOOS=windows go build -o receptor.exe cmd/receptor.go && \
	GOOS=darwin go build -o receptor.app cmd/receptor.go && \
	go build example/*.go

test: receptor
	@go test ./... -p 1 -parallel=16 -count=1

testloop: receptor
	@i=1; while echo "------ $$i" && \
	  go test ./... -p 1 -parallel=16 -count=1; do \
	  i=$$((i+1)); done

ci: pre-commit build-all test
	@echo "All done"

SPECFILES = packaging/rpm/receptor.spec packaging/rpm/receptorctl.spec

specfiles: $(SPECFILES)

$(SPECFILES): %.spec: %.spec.j2
	cat VERSION | jinja2 $< -o $@

VERSION = $(shell jq -r .version VERSION)
RELEASE = $(shell jq -r .release VERSION)
CONTAINERCMD ?= podman

# Other RPMs get built but we only track the main one for Makefile purposes
MAINRPM = rpmbuild/RPMS/x86_64/receptor-$(VERSION)-$(RELEASE).fc32.x86_64.rpm

$(MAINRPM): receptor $(SPECFILES)
	@$(CONTAINERCMD) build packaging/rpm-builder -t receptor-rpm-builder
	@$(CONTAINERCMD) run -it --rm -v $$PWD:/receptor:Z receptor-rpm-builder

rpms: $(MAINRPM)

container: rpms
	@cp -av rpmbuild/RPMS/ packaging/container/RPMS/
	@$(CONTAINERCMD) build packaging/container -t receptor

clean:
	@rm -fv receptor receptor.exe receptor.app net $(SPECFILES)
	@rm -rfv rpmbuild/
	@rm -rfv packaging/container/RPMS/

.PHONY: lint format fmt ci pre-commit build-all test clean testloop specfiles rpms container
