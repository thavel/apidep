IMAGE ?= apidep
TAG   ?= local

.PHONY: build run

build:
	docker build -t $(IMAGE):$(TAG) .

sync:
	docker run --rm \
		-v "$(shell pwd):/work" \
		-v "$(SSH_AUTH_SOCK):/ssh-agent" \
		-v "$(HOME)/.ssh/known_hosts:/root/.ssh/known_hosts:ro" \
		-e SSH_AUTH_SOCK=/ssh-agent \
		-e SSH_KNOWN_HOSTS=/root/.ssh/known_hosts \
		-w /work \
		$(IMAGE):$(TAG) sync
