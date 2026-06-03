BINARY := bin/laravel
PKG     := ./cmd/laravel

.PHONY: build run clean install fmt

build:
	go build -o $(BINARY) $(PKG)

run:
	go run $(PKG)

clean:
	rm -f bin/laravel bin/laravel-*

fmt:
	go fmt ./...

# Instala el binario en /usr/local/bin para usar "laravel" en cualquier sitio.
install: build
	cp $(BINARY) /usr/local/bin/laravel

# Compilación multiplataforma en bin/.
release:
	GOOS=linux  GOARCH=amd64 go build -o bin/laravel-linux-amd64   $(PKG)
	GOOS=darwin GOARCH=amd64 go build -o bin/laravel-darwin-amd64  $(PKG)
	GOOS=darwin GOARCH=arm64 go build -o bin/laravel-darwin-arm64  $(PKG)
