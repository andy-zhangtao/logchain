
.PHONY: build
name = logchain

build:
	go build -ldflags "-X main._VERSION_=$(shell date +%Y%m%d-%H%M%S)" -o $(name)

run: build
	./$(name)

release: *.go *.md rootfs
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main._VERSION_=$(shell date +%Y%m%d)" -a -o $(name)
	docker build -t vikings/$(name) .
	docker push vikings/$(name)

rootfs:
	@echo "### create rootfs directory in ./rootfs"
	mkdir -p ./rootfs
	docker create --name tmprootfs vikings/$(name)
	docker export tmprootfs | tar -x -C ./rootfs
	docker rm -vf tmprootfs