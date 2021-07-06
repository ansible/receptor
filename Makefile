# Calculate version number
# - If we are on an exact Git tag, then this is official and gets a -1 release
# - If we are not, then this is unofficial and gets a -0.date.gitref release
OFFICIAL_VERSION := $(shell if VER=`git describe --exact-match --tags 2>/dev/null`; then echo $$VER; else echo ""; fi)
VERSION := $(shell cd receptorctl && python3 setup.py --version)
ifeq ($(OFFICIAL_VERSION),)
RELEASE := 0.git$(shell date +'%Y%m%d').$(shell git rev-parse --short HEAD)
OFFICIAL :=
APPVER := $(VERSION)-$(RELEASE)
else
RELEASE := 1
OFFICIAL := yes
APPVER := $(VERSION)
endif

# Container command can be docker or podman
CONTAINERCMD ?= podman
TAG ?= receptor:latest

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
#
# no_cert_auth:   Disable commands related to CA and certificate generation

TAGS ?=
ifeq ($(TAGS),)
	TAGPARAM=
else
	TAGPARAM=--tags $(TAGS)
endif

receptor: $(shell find pkg -type f -name '*.go') cmd/receptor.go
	CGO_ENABLED=0 go build -ldflags "-X 'github.com/project-receptor/receptor/pkg/version.Version=$(APPVER)'" $(TAGPARAM) cmd/receptor.go

lint:
	@golint cmd/... pkg/... example/...

format:
	@find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

pre-commit:
	@pre-commit run --all-files

build-all:
	@echo "Running Go builds..." && \
	GOOS=windows go build -o receptor.exe cmd/receptor.go && \
	GOOS=darwin go build -o receptor.app cmd/receptor.go && \
	go build example/*.go && \
	go build --tags no_controlsvc,no_backends,no_services,no_tls_config,no_workceptor,no_cert_auth cmd/receptor.go && \
	go build cmd/receptor.go

RUNTEST ?=
ifeq ($(RUNTEST),)
TESTCMD =
else
TESTCMD = -run $(RUNTEST)
endif

test: receptor
	@go test ./... -p 1 -parallel=16 $(TESTCMD) -count=1

testloop: receptor
	@i=1; while echo "------ $$i" && \
	  go test ./... -p 1 -parallel=16 $(TESTCMD) -count=1; do \
	  i=$$((i+1)); done

kubectl:
	curl -LO "https://storage.googleapis.com/kubernetes-release/release/$$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
	chmod a+x kubectl

kubetest: kubectl
	./kubectl get nodes

ci: pre-commit build-all test receptorctl-tests
	@echo "All done"

version:
	@echo $(APPVER) > .VERSION
	@echo ".VERSION created for $(APPVER)"

SPECFILES = packaging/rpm/receptor.spec packaging/rpm/receptorctl.spec packaging/rpm/receptor-python-worker.spec

specfiles: $(SPECFILES)

$(SPECFILES): %.spec: %.spec.j2
	@jinja2 -D version=$(VERSION) -D release=$(RELEASE) $< -o $@

DIST = $(shell rpm --eval '%{dist}')
ARCH = $(shell rpm --eval '%{_arch}')

RPMSOURCEDIRS = cmd example pkg receptorctl receptor-python-worker packaging/rpm tests
RPMSOURCETAR = rpmbuild/SOURCES/receptor-$(VERSION).tar.gz
$(RPMSOURCETAR): $(shell find $(RPMSOURCEDIRS) -type f)
	mkdir receptor-$(VERSION)
	cp -av cmd/ example/ pkg/ receptorctl/ receptor-python-worker/ packaging/ tests/ receptor-$(VERSION)
	mkdir -p rpmbuild/SOURCES
	tar cfvz $(RPMSOURCETAR) receptor-$(VERSION)
	rm -rf receptor-$(VERSION)

RECEPTOR_RPM = rpmbuild/RPMS/x86_64/receptor-$(VERSION)-$(RELEASE)$(DIST).$(ARCH).rpm
RECEPTORCTL_RPM = rpmbuild/RPMS/noarch/receptorctl-$(VERSION)-$(RELEASE)$(DIST).noarch.rpm
RECEPTOR_PYTHON_WORKER_RPM = rpmbuild/RPMS/noarch/receptor-python-worker-$(VERSION)-$(RELEASE)$(DIST).noarch.rpm

RPMS = $(RECEPTOR_RPM) $(RECEPTORCTL_RPM) $(RECEPTOR_PYTHON_WORKER_RPM)

$(RECEPTOR_RPM): packaging/rpm/receptor.spec $(RPMSOURCETAR)
	rpmbuild -ba packaging/rpm/receptor.spec --without check -D "%_topdir $$PWD/rpmbuild"

$(RECEPTORCTL_RPM): packaging/rpm/receptorctl.spec $(RPMSOURCETAR)
	rpmbuild -ba packaging/rpm/receptorctl.spec -D "%_topdir $$PWD/rpmbuild"

$(RECEPTOR_PYTHON_WORKER_RPM): packaging/rpm/receptor-python-worker.spec $(RPMSOURCETAR)
	rpmbuild -ba packaging/rpm/receptor-python-worker.spec -D "%_topdir $$PWD/rpmbuild"

rpms: $(RPMS)

RECEPTORCTL_WHEEL = receptorctl/dist/receptorctl-$(VERSION)-py3-none-any.whl
$(RECEPTORCTL_WHEEL): receptorctl/README.md receptorctl/setup.py $(shell find receptorctl/receptorctl -type f -name '*.py')
	@cd receptorctl && python3 setup.py bdist_wheel

receptorctl_wheel: $(RECEPTORCTL_WHEEL)

RECEPTORCTL_SDIST = receptorctl/dist/receptorctl-$(VERSION).tar.gz
$(RECEPTORCTL_SDIST): receptorctl/README.md receptorctl/setup.py $(shell find receptorctl/receptorctl -type f -name '*.py')
	@cd receptorctl && python3 setup.py sdist

receptorctl_sdist: $(RECEPTORCTL_SDIST)

RECEPTOR_PYTHON_WORKER_WHEEL = receptor-python-worker/dist/receptor_python_worker-$(VERSION)-py3-none-any.whl
$(RECEPTOR_PYTHON_WORKER_WHEEL): receptor-python-worker/README.md receptor-python-worker/setup.py $(shell find receptor-python-worker/receptor_python_worker -type f -name '*.py')
	@cd receptor-python-worker && python3 setup.py bdist_wheel

container: .container-flag-$(VERSION)
.container-flag-$(VERSION): receptor $(RECEPTORCTL_WHEEL) $(RECEPTOR_PYTHON_WORKER_WHEEL)
	@tar --exclude-vcs-ignores -czf packaging/container/source.tar.gz .
	@cp $(RECEPTORCTL_WHEEL) packaging/container
	@cp $(RECEPTOR_PYTHON_WORKER_WHEEL) packaging/container
	$(CONTAINERCMD) build packaging/container --build-arg VERSION=$(VERSION) -t $(TAG) $(if $(OFFICIAL),-t receptor:$(VERSION),)
	@touch .container-flag-$(VERSION)

tc-image: container
	@cp receptor packaging/tc-image/
	@$(CONTAINERCMD) build packaging/tc-image -t receptor-tc

receptorctl-test-venv/bin/pytest:
	virtualenv receptorctl-test-venv -p python3
	receptorctl-test-venv/bin/pip install -e receptorctl
	receptorctl-test-venv/bin/pip install -r receptorctl/test-requirements.txt

receptorctl-tests: receptor receptorctl-test-venv/bin/pytest
	cd receptorctl && ../receptorctl-test-venv/bin/pytest tests/tests.py

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
	@rm -rfv receptorctl-test-venv/
	@rm -fv kubectl

.PHONY: lint format fmt ci pre-commit build-all test clean testloop specfiles rpms container version receptorctl-tests kubetest
