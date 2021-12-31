.PHONY: all push build deploy

all: build push deploy

push:
	kim push dukeman/wioc02

build:
	kim build -t dukeman/wioc02:latest .

deploy:
	@echo "Deploying manifest"
	kubectl kustomize | kubectl apply -f -