# list only our namespaced directories
PACKAGES = $(shell go list ./... | grep -v '/vendor/')

# Lint our code. Reference: https://golang.org/cmd/vet/
VETARGS?=-asmdecl -atomic -bool -buildtags -copylocks -methods \
         -nilfunc -printf -rangeloops -shift -structtags -unsafeptr

# Tools required for different make targets or for development purposes
EXTERNAL_TOOLS=\
	github.com/kardianos/govendor \
	github.com/mitchellh/gox \
	golang.org/x/tools/cmd/cover \
	github.com/axw/gocov/gocov \
	gopkg.in/matm/v1/gocov-html \
	github.com/ugorji/go/codec/codecgen

# list only our .go files i.e. exlcudes any .go files from the vendor directory
GOFILES_NOVENDOR = $(shell find . -type f -name '*.go' -not -path "./vendor/*")

# Specify the name for the maya server binary
CTLNAME=mayaserver

all: test

dev: format
	@CTLNAME=${CTLNAME} MAYASERVER_DEV=1 sh -c "'$(PWD)/buildscripts/build.sh'"

bin:
	@CTLNAME=${CTLNAME} sh -c "'$(PWD)/buildscripts/build.sh'"

init: bootstrap deps

deps:
	rm -rf vendor/github.com/
	@echo "--> Sync with vendored repositories." ;
	@echo "--> Please wait, this may take a while..." ;
	@govendor sync
	cp -r ./vendor/github.com/kubernetes/kubernetes/staging/src/k8s.io/ ./vendor/

sync:
	@govendor sync
	cp -r ./vendor/github.com/kubernetes/kubernetes/staging/src/k8s.io/ ./vendor/

clean:
	rm -rf bin
	rm -rf ${GOPATH}/bin/${CTLNAME}

release:
	@$(MAKE) bin

# Run the bootstrap target once before trying cov
cov:
	gocov test ./... | gocov-html > /tmp/coverage.html
	@cat /tmp/coverage.html

test:
	@echo "--> Running go fmt" ;
	@if [ -n "`go fmt ${PACKAGES}`" ]; then \
		echo "[ERR] go fmt updated formatting. Please commit formatted code first."; \
		exit 1; \
	fi
	@CTLNAME=${CTLNAME} sh -c "'$(PWD)/buildscripts/test.sh'"

cover:
	go list ./... | grep -v vendor | xargs -n1 go test --cover

format:
	@echo "--> Running go fmt"
	@go fmt $(PACKAGES)

vet:
	@go tool vet 2>/dev/null ; if [ $$? -eq 3 ]; then \
		go get golang.org/x/tools/cmd/vet; \
	fi
	@go tool vet $(VETARGS) ${GOFILES_NOVENDOR} ; if [ $$? -eq 1 ]; then \
		echo ""; \
		echo "[LINT] Vet found suspicious constructs."; \
		echo "Fix them if necessary before submitting the code for review."; \
	fi

	@git grep -n `echo "log"".Print"` | grep -v 'vendor/' ; if [ $$? -eq 0 ]; then \
		echo "[LINT] Found "log"".Printf" calls. These should use Mayaserver's logger instead."; \
	fi

# Bootstrap the build by downloading additional tools
bootstrap:
	@for tool in  $(EXTERNAL_TOOLS) ; do \
		echo "Installing $$tool" ; \
		go get $$tool; \
	done 

# You might need to use sudo
install: bin/${CTLNAME}
	install -o root -g root -m 0755 ./bin/${CTLNAME} /usr/local/bin/${CTLNAME}

.PHONY: all bin cov install test vet format cover bootstrap release clean deps init dev sync
