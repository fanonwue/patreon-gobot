# Add linker flags when building for production
# those will strip symbols and debug info
# both "prod" and "production" count as production target
PROD_TARGETS=prod production
ifneq (, $(filter $(TARGET), $(PROD_TARGETS)))
	GO_TAGS := release
	LD_FLAGS := -w -s
else
	GO_TAGS := debug
endif

EXECUTABLE_NAME := patreon-gobot
CC := cc
ifeq ($(GOOS), windows)
	EXECUTABLE_NAME := $(EXECUTABLE_NAME).exe
	CC := x86_64-w64-mingw32-gcc-win32
endif

# Enabling CGO is required for sqlite, the Go package is just a stub
build:
	CGO_ENABLED=1 CC=$(CC) go build -tags $(GO_TAGS) -o bin/$(EXECUTABLE_NAME) --ldflags="$(LD_FLAGS)"

deps:
	go mod download && go mod verify

deps-update:
	go get -u && go mod tidy