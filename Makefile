help:
	@echo "Please use 'make <target>' where <target> is one of the following:"
	@echo "  test                 to run unit tests."
	@echo "  build                to build the app as a binary."
	@echo "  build-image          to build the app container."
	@echo "  run                  to run the app with go."

run:
	go run -ldflags="$(govvv -flags -pkg) -w -s" ./cmd/casper-3/main.go

test:
	go test -v -coverpkg=./... -coverprofile=profile.cov ./...
	go tool cover -func profile.cov
	rm profile.cov

build:
	CGO_ENABLED=0 go build -mod=readonly -ldflags="$(govvv -flags -pkg $(go list ./info)) -w -s" -o ./bin/casper-3 ./cmd/casper-3/*

build-image:
	docker build -t gathertown/casper-3:latest
