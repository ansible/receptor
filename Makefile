# If the current commit has been tagged, use that as the version.
# Otherwise include short commit hash.
OFFICIAL_VERSION := $(shell if VER=`git describe --exact-match --tags 2>/dev/null`; then echo $$VER; else echo ""; fi)
ifeq ($(OFFICIAL_VERSION),)
VERSION := $(shell git describe --tags | cut -d - -f -1)+g$(shell git rev-parse --short HEAD)
else
VERSION := $(OFFICIAL_VERSION)
endif


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

DEBUG ?=
ifeq ($(DEBUG),1)
	DEBUGFLAGS=-gcflags=all="-N -l"
else
	DEBUGFLAGS=
endif

receptor: $(shell find pkg -type f -name '*.go') ./cmd/receptor-cl/receptor.go
	CGO_ENABLED=0 go build -o receptor $(DEBUGFLAGS) -ldflags "-X 'github.com/ansible/receptor/internal/version.Version=$(VERSION)'" $(TAGPARAM) ./cmd/receptor-cl

lint:
	@golint cmd/... pkg/... example/...

receptorctl-lint: receptorctl/.VERSION
	@cd receptorctl && tox -e lint

format:
	@find cmd/ pkg/ -type f -name '*.go' -exec go fmt {} \;

fmt: format

pre-commit:
	@pre-commit run --all-files

build-all:
	@echo "Running Go builds..." && \
	GOOS=windows go build -o receptor.exe ./cmd/receptor-cl && \
	GOOS=darwin go build -o receptor.app ./cmd/receptor-cl && \
	go build example/*.go && \
	go build -o receptor --tags no_controlsvc,no_backends,no_services,no_tls_config,no_workceptor,no_cert_auth ./cmd/receptor-cl && \
	go build -o receptor ./cmd/receptor-cl

RUNTEST ?=
ifeq ($(RUNTEST),)
TESTCMD =
else
TESTCMD = -run $(RUNTEST)
endif

test: receptor
	PATH=${PWD}:${PATH} \
	go test ./... -p 1 -parallel=16 $(TESTCMD) -count=1 -race

receptorctl-test: receptorctl/.VERSION
	@cd receptorctl && tox -e py3

testloop: receptor
	@i=1; while echo "------ $$i" && \
	  go test ./... -p 1 -parallel=16 $(TESTCMD) -count=1; do \
	  i=$$((i+1)); done

kubectl:
	curl -LO "https://storage.googleapis.com/kubernetes-release/release/$$(curl -s https://storage.googleapis.com/kubernetes-release/release/stable.txt)/bin/linux/amd64/kubectl"
	chmod a+x kubectl

kubetest: kubectl
	./kubectl get nodes

version:
	@echo $(VERSION) > .VERSION
	@echo ".VERSION created for $(VERSION)"

receptorctl/.VERSION:
	echo $(VERSION) > $@

RECEPTORCTL_WHEEL = receptorctl/dist/receptorctl-$(VERSION:v%=%)-py3-none-any.whl
$(RECEPTORCTL_WHEEL): receptorctl/README.md receptorctl/.VERSION $(shell find receptorctl/receptorctl -type f -name '*.py')
	@cd receptorctl && python3 -m build --wheel

receptorctl_wheel: $(RECEPTORCTL_WHEEL)

RECEPTORCTL_SDIST = receptorctl/dist/receptorctl-$(VERSION:v%=%).tar.gz
$(RECEPTORCTL_SDIST): receptorctl/README.md receptorctl/.VERSION $(shell find receptorctl/receptorctl -type f -name '*.py')
	@cd receptorctl && python3 -m build --sdist

receptorctl_sdist: $(RECEPTORCTL_SDIST)

receptor-python-worker/.VERSION:
	echo $(VERSION) > $@

RECEPTOR_PYTHON_WORKER_WHEEL = receptor-python-worker/dist/receptor_python_worker-$(VERSION:v%=%)-py3-none-any.whl
$(RECEPTOR_PYTHON_WORKER_WHEEL): receptor-python-worker/README.md receptor-python-worker/.VERSION $(shell find receptor-python-worker/receptor_python_worker -type f -name '*.py')
	@cd receptor-python-worker && python3 -m build --wheel

# Container command can be docker or podman
CONTAINERCMD ?= podman

# Repo without tag
REPO := quay.io/ansible/receptor
# TAG is VERSION with a '-' instead of a '+', to avoid invalid image reference error.
TAG := $(subst +,-,$(VERSION))
# Set this to tag image as :latest in addition to :$(VERSION)
LATEST :=

EXTRA_OPTS ?=

container: .container-flag-$(VERSION)
.container-flag-$(VERSION): $(RECEPTORCTL_WHEEL) $(RECEPTOR_PYTHON_WORKER_WHEEL)
	@tar --exclude-vcs-ignores -czf packaging/container/source.tar.gz .
	@cp $(RECEPTORCTL_WHEEL) packaging/container
	@cp $(RECEPTOR_PYTHON_WORKER_WHEEL) packaging/container
	$(CONTAINERCMD) build $(EXTRA_OPTS) packaging/container --build-arg VERSION=$(VERSION:v%=%) -t $(REPO):$(TAG) $(if $(LATEST),-t $(REPO):latest,)
	@touch .container-flag-$(VERSION)

tc-image: container
	@cp receptor packaging/tc-image/
	@$(CONTAINERCMD) build packaging/tc-image -t receptor-tc

clean:
	@rm -fv receptor receptor.exe receptor.app net
	@rm -rfv packaging/container/RPMS/
	@rm -fv receptorctl/dist/*
	@rm -fv receptor-python-worker/dist/*
	@rm -fv packaging/container/receptor
	@rm -fv packaging/container/*.whl
	@rm -fv .container-flag*
	@rm -fv .VERSION
	@rm -rfv receptorctl-test-venv/
	@rm -fv kubectl

.PHONY: lint format fmt pre-commit build-all test clean testloop container version receptorctl-tests kubetest receptorctl/.VERSION receptor-python-worker/.VERSION
