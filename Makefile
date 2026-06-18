# MAKEFILE FOR ONGOPLAYER
# Build targets for Linux and Windows, GUI (Gio) and WebUI modes
# Usage:
#   make dev          - Run GUI in dev mode (Linux)
#   make dev-webui    - Run WebUI in dev mode
#   make build        - Build all Linux binaries
#   make build-windows- Cross-compile all Windows binaries
#   make clean        - Remove build artifacts

APP_NAME  := OngoPlayer
BUILD_DIR := build
WAYVULKAN_TAGS := nox11,noopengl
WAYOPENGL_TAGS := nox11
X11GL_TAGS := nowayland

.PHONY: dev dev-debug dev-webui

dev:
	SDL_AUDIODRIVER=pulseaudio go run --tags "nowayland noopengl" cmd/gui/main.go --debug

dev-wayvulkan:
	SDL_AUDIODRIVER=pulseaudio go run --tags nox11,noopengl cmd/gui/main.go  --debug --playlist "/home/kasaki/Music/agak fayer"

dev-webui:
	cd cmd/webui && wails dev

dev-windows: 
	go run cmd/gui/main.go --playlist "C:/Users/Hitori/Music/Mix random jpop" --rpc --debug

.PHONY: build build-linux-gui build-webview-gui build-dsp-linux build-dsp-windows

build-dsp-linux:
	mkdir -p $(BUILD_DIR)
	$(CC) -O3 -shared -fPIC -ffast-math -o $(BUILD_DIR)/stelle_dsp.so Audioengine/StelleEngine/dsp/stelle_dsp.c

build-dsp-windows:
	mkdir -p build/win
	gcc -O3 -shared -ffast-math -o build/win/stelle_dsp.dll Audioengine/StelleEngine/dsp/stelle_dsp.c

build: build-dsp-linux build-linux-gui

build-linux-gui:
	GOOS=linux GOARCH=amd64 go build $(WAYVULKAN_TAGS) \
		-o $(BUILD_DIR)/$(APP_NAME)-gui ./cmd/gui

.PHONY: build-windows build-windows-gui build-webview-gui

build-windows: build-dsp-windows build-windows-gui build-webview-gui

build-windows-gui:
	GOOS=windows GOARCH=amd64 go build \
		-o $(BUILD_DIR)/$(APP_NAME)-gui.exe ./cmd/gui

build-windows-release:
	gogio -target windows -icon .\cmd\gui\assets\appicon.png -o build\win\OngoPlayer.exe ./cmd/gui
	
build-webview-gui:
	cd cmd/webui && wails build -platform windows/amd64 -o ../../$(BUILD_DIR)/$(APP_NAME)-webview.exe


.PHONY: all
all: build build-windows
	
.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)