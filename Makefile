# Calculate version number
# - If we are on an exact Git tag, then this is official and gets a -1 release
# - If we are not, then this is unofficial and gets a -0.date.gitref release
OFFICIAL_VERSION = $(shell if VER=`git describe --exact-match --tags 2>/dev/null`; then echo $$VER; else echo ""; fi)
ifeq ($(OFFICIAL_VERSION),)
VERSION = $(shell git describe --tags | sed 's/-.*//' | awk -F. -v OFS=. '{$$NF++;print}')
RELEASE = 0.git$(shell date -u +%Y%m%d%H%M).$(shell git rev-parse --short HEAD)
OFFICIAL =
APPVER = $(VERSION)-$(RELEASE)
else
VERSION = $(OFFICIAL_VERSION)
RELEASE = 1
OFFICIAL = yes
APPVER = $(VERSION)
endif

# Container command can be docker or podman
CONTAINERCMD ?= podman

# When building Receptor, tags can be used to remove undesired
# features.  This is primarily used for deploying Receptor in a
# security sensitive role, where it is desired to have no possibility
# of a service being accidentally enabled.  Features are controlled
# using the TAGS environment variable, which is a comma delimeted
# list of zero or more of the following:
#
# no_controlsvc:  Disable the control service
#
# no_backends:    Disable all backends (except external via the API)
# no_tcp_backend: Disable the TCP backend
# no_udp_backend: Disable the UDP backend
# no_websocket_backend: Disable the websocket backent
#
# no_services:    Disable all services
# no_proxies:     Disable the TCP, UDP and Unix proxy services
# no_ip_router:   Disable the IP router service
#
# no_tls_config:  Disable the ability to configure TLS server/client configs
#
# no_workceptor:  Disable the unit-of-work subsystem (be network only)

TAGS ?=
ifeq ($(TAGS),)
	TAGPARAM=
else
	TAGPARAM=--tags $(TAGS)
endif

receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	go build -ldflags "-X main.version=$(APPVER)" $(TAGPARAM) cmd/receptor.go

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
	go build example/*.go && \
	go build --tags no_controlsvc,no_backends,no_services,no_tls_config,no_workceptor cmd/receptor.go && \
	go build cmd/receptor.go

test: receptor
	@go test ./... -p 1 -parallel=16 -count=1

RUNTEST ?=
ifeq ($(RUNTEST),)
TESTCMD =
else
TESTCMD = -run $(RUNTEST)
endif

testloop: receptor
	@i=1; while echo "------ $$i" && \
	  go test ./... -p 1 -parallel=16 $(TESTCMD) -count=1; do \
	  i=$$((i+1)); done

ci: pre-commit build-all test
	@echo "All done"

version:
	@echo $(VERSION) > .VERSION
	@echo ".VERSION created for $(VERSION)"

SPECFILES = packaging/rpm/receptor.spec packaging/rpm/receptorctl.spec packaging/rpm/receptor-python-worker.spec

specfiles: $(SPECFILES)

$(SPECFILES): %.spec: %.spec.j2
	@jinja2 -D version=$(VERSION) -D release=$(RELEASE) $< -o $@

# Other RPMs get built but we only track the main one for Makefile purposes
MAINRPM = rpmbuild/RPMS/x86_64/receptor-$(VERSION)-$(RELEASE).fc32.x86_64.rpm

.rpm-builder-flag: $(shell find packaging/rpm-builder -type f)
	@$(CONTAINERCMD) build packaging/rpm-builder -t receptor-rpm-builder
	@touch .rpm-builder-flag

$(MAINRPM): .rpm-flag-$(VERSION)
.rpm-flag-$(VERSION): receptor $(SPECFILES) .rpm-builder-flag
	@echo $(VERSION) > .VERSION
	@$(CONTAINERCMD) run -it --rm -v $$PWD:/receptor:Z receptor-rpm-builder
	@touch .rpm-flag-$(VERSION)

rpms: $(MAINRPM)

RECEPTORCTL_WHEEL = receptorctl/dist/receptorctl-$(VERSION)-py3-none-any.whl
$(RECEPTORCTL_WHEEL): receptorctl/README.md receptorctl/setup.py $(shell find receptorctl/receptorctl -type f -name '*.py')
	@echo $(VERSION) > .VERSION
	@cd receptorctl && python3 setup.py bdist_wheel

RECEPTOR_PYTHON_WORKER_WHEEL = receptor-python-worker/dist/receptor_python_worker-$(VERSION)-py3-none-any.whl
$(RECEPTOR_PYTHON_WORKER_WHEEL): receptor-python-worker/README.md receptor-python-worker/setup.py $(shell find receptor-python-worker/receptor_python_worker -type f -name '*.py')
	@echo $(VERSION) > .VERSION
	@cd receptor-python-worker && python3 setup.py bdist_wheel

container: .container-flag-$(VERSION)
.container-flag-$(VERSION): receptor $(RECEPTORCTL_WHEEL) $(RECEPTOR_PYTHON_WORKER_WHEEL)
	@cp receptor packaging/container
	@cp $(RECEPTORCTL_WHEEL) packaging/container
	@cp $(RECEPTOR_PYTHON_WORKER_WHEEL) packaging/container
	$(CONTAINERCMD) build packaging/container --build-arg VERSION=$(VERSION) -t receptor:latest $(if $(OFFICIAL),-t receptor:$(VERSION),)
	@touch .container-flag-$(VERSION)

tc-image: container
	@cp receptor packaging/tc-image/
	@$(CONTAINERCMD) build packaging/tc-image -t receptor-tc

clean:
	@rm -fv receptor receptor.exe receptor.app net $(SPECFILES)
	@rm -rfv rpmbuild/
	@rm -rfv packaging/container/RPMS/
	@rm -fv receptorctl/dist/*
	@rm -fv receptor-python-worker/dist/*
	@rm -fv packaging/container/receptor
	@rm -fv packaging/container/*.whl
	@rm -fv .container-flag* .rpm-flag* .rpm-builder-flag
	@rm -fv .VERSION

.PHONY: lint format fmt ci pre-commit build-all test clean testloop specfiles rpms container
