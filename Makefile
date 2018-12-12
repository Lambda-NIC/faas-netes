TAG?=latest
NS?=lambdanic
REPO?=faas-netes

all: build

local:
	CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o faas-netes

build:
	docker build --build-arg http_proxy="${http_proxy}" --build-arg https_proxy="${https_proxy}" -t $(NS)/$(REPO):$(TAG) .

push:
	docker push $(NS)/$(REPO):$(TAG)

namespaces:
	kubectl apply -f namespaces.yml

install: namespaces
	kubectl apply -f yaml/
