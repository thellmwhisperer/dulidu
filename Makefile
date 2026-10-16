.PHONY: test lint release dist

test:
	go build ./...
	go test -count=1 ./...

lint:
	go vet ./...
	test -z "$$(gofmt -l cmd internal)"

PKG := github.com/thellmwhisperer/dulidu/internal/buildinfo
override TAG := $(value TAG)
export TAG
release:
	go run ./cmd/dulidu release "$$TAG"

dist:
	test -n "$$TAG" || { echo "usage: make dist TAG=vX.Y.Z" >&2; exit 2; }
	rm -rf dist && mkdir dist
	for p in darwin/amd64 darwin/arm64 linux/amd64 linux/arm64; do \
	  os=$${p%/*} arch=$${p#*/}; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o dist/dulidu -ldflags "-X $(PKG).Version=$$TAG \
	    -X $(PKG).Commit=$$(git rev-parse --short=7 HEAD) -X $(PKG).Date=$$(date -u +%Y-%m-%dT%H:%M:%SZ)" ./cmd/dulidu && \
	  tar -C dist -czf dist/dulidu-$$TAG-$$os-$$arch.tar.gz dulidu || exit 1; \
	done
	rm dist/dulidu
	cd dist && (command -v sha256sum >/dev/null && sha256sum dulidu-* || shasum -a 256 dulidu-*) > checksums.txt
