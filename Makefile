# project specific definitions
SRCDIR = cmd
BINDIR = bin
PKGDOCKER = virgo4-pool-solr-ws
PACKAGES = $(PKGDOCKER)

# go commands
GOCMD = go
GOBLD = $(GOCMD) build
GOCLN = $(GOCMD) clean
GOTST = $(GOCMD) test
GOVET = $(GOCMD) vet
GOFMT = $(GOCMD) fmt
GOGET = $(GOCMD) get
GOMOD = $(GOCMD) mod
GOVER = $(GOCMD) version
GOLNT = golint

# default build target is host machine architecture
MACHINE = $(shell uname -s | tr '[A-Z]' '[a-z]')
TARGET = $(MACHINE)

# git commit used for this build, either passed to make via Dockerfile or determined from local directory
ifeq ($(GIT_COMMIT),)
	GIT_COMMIT = $(shell commit="$$(git rev-list -1 HEAD)" ; postfix="" ; git diff --quiet || postfix="-modified" ; echo "$${commit}$${postfix}")
endif

# darwin-specific definitions
GOENV_darwin = 
GOFLAGS_darwin = 
GOLINK_darwin = 

# linux-specific definitions
GOENV_linux = 
GOFLAGS_linux = 
GOLINK_linux = 

# extra flags
GOENV_EXTRA = GOARCH=amd64
GOFLAGS_EXTRA =
GOLINK_EXTRA = -X main.gitCommit=$(GIT_COMMIT)

# default target:

build: go-vars compile symlink

go-vars:
	$(eval GOENV = GOOS=$(TARGET) $(GOENV_$(TARGET)) $(GOENV_EXTRA))
	$(eval GOFLAGS = $(GOFLAGS_$(TARGET)) $(GOFLAGS_EXTRA))
	$(eval GOLINK = -ldflags '$(GOLINK_$(TARGET)) $(GOLINK_EXTRA)')

compile:
	@ \
	echo "building packages: [$(PACKAGES)] for target: [$(TARGET)]" ; \
	echo ; \
	$(GOVER) ; \
	echo ; \
	for pkg in $(PACKAGES) ; do \
		printf "compile: %-6s  env: [%s]  flags: [%s]  link: [%s]\n" "$${pkg}" "$(GOENV)" "$(GOFLAGS)" "$(GOLINK)" ; \
		$(GOENV) $(GOBLD) $(GOFLAGS) $(GOLINK) -o "$(BINDIR)/$${pkg}.$(TARGET)" "$(SRCDIR)/$${pkg}"/*.go || exit 1 ; \
	done

symlink:
	@ \
	echo ; \
	for pkg in $(PACKAGES) ; do \
		echo "symlink: $(BINDIR)/$${pkg} -> $${pkg}.$(TARGET)" ; \
		ln -sf "$${pkg}.$(TARGET)" "$(BINDIR)/$${pkg}" || exit 1 ; \
	done

darwin: target-darwin build

target-darwin:
	$(eval TARGET = darwin)

linux: target-linux build

target-linux:
	$(eval TARGET = linux)

rebuild: flag build

flag:
	$(eval GOFLAGS_EXTRA += -a)

rebuild-darwin: target-darwin rebuild

rebuild-linux: target-linux rebuild

# docker: make sure binary is linux and truly static
docker-vars:
	$(eval PACKAGES = $(PKGDOCKER))
	$(eval GOENV_EXTRA += CGO_ENABLED=0)
	$(eval GOLINK_EXTRA += -extldflags "-static")

docker: docker-vars linux

rebuild-docker: docker-vars rebuild-linux

# maintenance rules
fmt:
	@ \
	for pkg in $(PACKAGES) ; do \
		echo "fmt: $${pkg}" ; \
		(cd "$(SRCDIR)/$${pkg}" && $(GOFMT)) ; \
	done

vet:
	@ \
	for pkg in $(PACKAGES) ; do \
		echo "vet: $${pkg}" ; \
		(cd "$(SRCDIR)/$${pkg}" && $(GOVET)) ; \
	done

lint:
	@ \
	for pkg in $(PACKAGES) ; do \
		echo "lint: $${pkg}" ; \
		(cd "$(SRCDIR)/$${pkg}" && $(GOLNT)) ; \
	done

check: fmt vet lint

clean:
	@ \
	echo "purge: $(BINDIR)/" ; \
	rm -rf $(BINDIR) ; \
	for pkg in $(PACKAGES) ; do \
		echo "clean: $${pkg}" ; \
		(cd "$(SRCDIR)/$${pkg}" && $(GOCLN)) ; \
	done

dep:
	$(GOGET) -u
	$(GOMOD) tidy
	$(GOMOD) verify
