
.PHONY: build
name = logchain

build:
	go build -ldflags "-X main._VERSION_=$(shell date +%Y%m%d-%H%M%S)" -o $(name)

run: build
	./$(name)

docker: *.go *md
	GOOS=linux GOARCH=amd64 go build -ldflags "-X main._VERSION_=$(shell date +%Y%m%d)" -a -o $(name)
	docker build -t vikings/$(name) .

release: docker
	docker push vikings/$(name)

rootfs: release
	@echo "### create rootfs directory in ./rootfs"
	mkdir -p ./rootfs
	docker create --name tmprootfs vikings/$(name)
	docker export tmprootfs | tar -x -C ./rootfs
	docker rm -vf tmprootfs