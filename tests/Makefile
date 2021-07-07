CONTAINER_IMAGE_TAG_BASE=ansible/receptor:base
CONTAINER_IMAGE_TAG_TEST=ansible/receptor:test

# Tests
GO_FUNCTIONAL_TESTS_DIRS:=$$(find ./functional -type d)
tests:
	for f in $(GO_FUNCTIONAL_TESTS_DIRS) ; do \
		go test -v $$f ; \
	done

# Generate artifacts
artifacts: container-image-base
	mkdir -p $(PWD)/artifacts
	podman run -it --rm \
		-v $(PWD)/../:/source/:ro \
		-v $(PWD)/artifacts/:/artifacts/:rw \
		$(CONTAINER_IMAGE_TAG_BASE) \
		/build-artifacts.sh

# Container environment
container-image: container-image-base

container-image-base:
	podman build -t $(CONTAINER_IMAGE_TAG_BASE) -f ./environments/container/Containerfile ./../

# container-image-test:
# 	podman build -t $(CONTAINER_IMAGE_TAG_BASE) -f ./environments/container/Containerfile ./../

container-shell-base:
	podman run -it --rm \
		-v $(PWD)/../:/source/:ro \
		$(CONTAINER_IMAGE_TAG_BASE) sh -c \
			'cp -r /source /build && cd /build && bash'

# container-shell-test:
# 	podman run -it --rm \
# 		-v $(PWD)/../:/source/:ro \
# 		$(CONTAINER_IMAGE_TAG_BASE) bash

# VM (vagrant)
vm: vm-create

vm-create:
	cd environments/vagrant && \
	vagrant up

shell: vm-shell
vm-shell:
	@cd environments/vagrant && \
	vagrant ssh -c "cd /vagrant && /bin/bash"

vm-destroy:
	cd environments/vagrant && \
	vagrant destroy
