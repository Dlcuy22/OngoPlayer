# MAKEFILE FOR ONGOPLAYER
# Build targets for Linux and Windows, GUI (Gio) and TUI modes
# Usage:
#   make dev          - Run GUI in dev mode (Linux)
#   make dev-tui      - Run TUI in dev mode (Linux)
#   make build        - Build all Linux binaries
#   make build-windows- Cross-compile all Windows binaries
#   make clean        - Remove build artifacts

APP_NAME  := OngoPlayer
BUILD_DIR := build
WAYVULKAN_TAGS := nox11,noopengl
WAYOPENGL_TAGS := nox11
X11GL_TAGS := nowayland

.PHONY: dev dev-debug dev-tui

dev:
	SDL_AUDIODRIVER=pulseaudio go run --tags "nowayland noopengl" cmd/gui/main.go --debug

dev-wayvulkan:
	SDL_AUDIODRIVER=pulseaudio go run --tags nox11,noopengl cmd/gui/main.go  --debug --playlist "/home/kasaki/Music/agak fayer"

dev-tui:
	go run cmd/tui/main.go



.PHONY: build build-linux-gui build-linux-tui

build: build-linux-gui build-linux-tui

build-linux-gui:
	GOOS=linux GOARCH=amd64 go build $(WAYVULKAN_TAGS) \
		-o $(BUILD_DIR)/$(APP_NAME)-gui ./cmd/gui

build-linux-tui:
	GOOS=linux GOARCH=amd64 go build \
		-o $(BUILD_DIR)/$(APP_NAME)-tui ./cmd/tui

.PHONY: build-windows build-windows-gui build-windows-tui

build-windows: build-windows-gui build-windows-tui

build-windows-gui:
	GOOS=windows GOARCH=amd64 go build \
		-o $(BUILD_DIR)/$(APP_NAME)-gui.exe ./cmd/gui

build-windows-tui:
	GOOS=windows GOARCH=amd64 go build \
		-o $(BUILD_DIR)/$(APP_NAME)-tui.exe ./cmd/tui


.PHONY: all
all: build build-windows
	
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)