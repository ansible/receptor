CONTAINER_IMAGE_TAG_builder=ansible/receptor:builder
CONTAINER_IMAGE_TAG_dev=ansible/receptor:dev

CONTAINER_RUN:=$$(./receptor-tester.sh container-runtime)

# Tests
GO_FUNCTIONAL_TESTS_DIRS:=$$(find ./functional -type d)

# Generate artifacts
artifacts: container-image-builder
	mkdir -p $(PWD)/artifacts-output
	$(CONTAINER_RUN) run --rm \
		-v $(PWD)/../:/source/ \
		-v $(PWD)/artifacts-output/:/artifacts/:rw \
		-v receptor_go_root_cache:/root/go:rw \
		-e OUTPUT_UID=$$(id -u) \
		$(CONTAINER_IMAGE_TAG_builder) \
		/build-artifacts.sh

artifacts-no-cache: container-image-builder
	mkdir -p $(PWD)/artifacts
	$(CONTAINER_RUN) run --rm \
		-v $(PWD)/../:/source/:ro \
		-v $(PWD)/artifacts/:/artifacts/:rw \
		$(CONTAINER_IMAGE_TAG_builder) \
		/build-artifacts.sh

# Container environment
container-image-dev:
	$(CONTAINER_RUN) build \
		-t $(CONTAINER_IMAGE_TAG_dev) \
		-f ./environments/container-dev/Containerfile \
		./../

container-image-builder:
	$(CONTAINER_RUN) build \
		-t $(CONTAINER_IMAGE_TAG_builder) \
		-f ./environments/container-builder/Containerfile \
		./../

container-image-dev-shell:
	$(CONTAINER_RUN) run -it --rm \
		$(CONTAINER_IMAGE_TAG_dev) bash

container-image-builder-shell:
	$(CONTAINER_RUN) run -it --rm \
		-v $(PWD)/../:/source/:ro \
		-v receptor_go_root_cache:/root/go:rw \
		$(CONTAINER_IMAGE_TAG_builder) bash -c \
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
