CONTAINER_IMAGE_TAG_BASE=ansible/receptor:base
CONTAINER_IMAGE_TAG_TEST=ansible/receptor:test

CONTAINER_RUN=podman

# Tests
GO_FUNCTIONAL_TESTS_DIRS:=$$(find ./functional -type d)

# Generate artifacts
artifacts: container-image-base
	mkdir -p $(PWD)/artifacts
	$(CONTAINER_RUN) run --rm \
		-v $(PWD)/../:/source/:ro \
		-v $(PWD)/artifacts/:/artifacts/:rw \
		-v receptor_go_root_cache:/root/go:rw \
		$(CONTAINER_IMAGE_TAG_BASE) \
		/build-artifacts.sh

artifacts-no-cache: container-image-base
	mkdir -p $(PWD)/artifacts
	$(CONTAINER_RUN) run --rm \
		-v $(PWD)/../:/source/:ro \
		-v $(PWD)/artifacts/:/artifacts/:rw \
		$(CONTAINER_IMAGE_TAG_BASE) \
		/build-artifacts.sh

# Container environment
container-image: container-image-base

container-image-base:
	$(CONTAINER_RUN) build -t $(CONTAINER_IMAGE_TAG_BASE) -f ./environments/container/Containerfile ./../

container-shell-base:
	$(CONTAINER_RUN) run -it --rm \
		-v $(PWD)/../:/source/:ro \
		-v receptor_go_root_cache:/root/go:rw \
		$(CONTAINER_IMAGE_TAG_BASE) bash -c \
			'cp -r /source /build; cd /build; bash'

# VM (vagrant)
vm: vm-create

vm-create:
	cd environments/vagrant && \
	vagrant up

shell: vm-shell
vm-shell:
	@cd environments/vagrant && \
	vagrant ssh -c "cd /vagrant/tests && /bin/bash"

vm-destroy:
	cd environments/vagrant && \
	vagrant destroy

vm-provision:
	cd environments/vagrant && \
	vagrant provision

# Colored output
ccblue=$(echo -e "\033[0;31m")
ccred=$(echo -e "\033[0;31m")
ccyellow=$(echo -e "\033[0;33m")
ccend=$(echo -e "\033[0m")

# Other configs
SHELL:=/bin/bash
