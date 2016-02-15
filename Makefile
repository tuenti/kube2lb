VERSION := 0.9

all:
	go build -ldflags "-X main.version=$(VERSION)"

release:
	@if echo $(VERSION) | grep -q "dev$$" ; then echo Set VERSION variable to release; exit 1; fi
	@if git show v$(VERSION) > /dev/null 2>&1; then echo Version $(VERSION) already exists; exit 1; fi
	sed -i "s/^VERSION :=.*/VERSION := $(VERSION)/" Makefile
	git ci Makefile -m "Version $(VERSION)"
	git tag v$(VERSION) -a -m "Version $(VERSION)"
	go build -ldflags "-X main.version=$(VERSION)"
	git checkout HEAD^ Makefile
	git ci Makefile -m "Starting next version"
