# MAKEFILE FOR ONGOPLAYER

default: build

BUILD_DIR := build/bin
WEBUI_DIR  := cmd/webui
UNAME_S := $(shell uname -s)
CC := gcc
ifeq ($(UNAME_S),Linux)
APP_NAME  := OngoPlayer
DSP_EXT    := .so
DSP_TARGET := stelle_dsp$(DSP_EXT)
DSP_FLAGS  := -fPIC
WTAGS     := -tags webkit2_41
endif

ifneq (,$(findstring MINGW,$(UNAME_S))$(findstring MSYS,$(UNAME_S)))
APP_NAME  := OngoPlayer.exe
DSP_EXT    := .dll
DSP_TARGET := stelle_dsp$(DSP_EXT)
DSP_FLAGS  :=
WTAGS     :=
endif

ifeq ($(UNAME_S),Windows_NT)
APP_NAME  := OngoPlayer.exe
DSP_EXT    := .dll
DSP_TARGET := stelle_dsp$(DSP_EXT)
DSP_FLAGS  :=
WTAGS     :=
endif

ifeq ($(UNAME_S),Darwin)
APP_NAME  := OngoPlayer
DSP_EXT    := .dylib
DSP_TARGET := stelle_dsp$(DSP_EXT)
DSP_FLAGS  := -fPIC
WTAGS     :=
endif


dev:
	$(MAKE) build-dsp
	cd $(WEBUI_DIR) && wails dev $(WTAGS)

build-dsp:
	mkdir -p $(BUILD_DIR)
	$(CC) -O3 -shared $(DSP_FLAGS) -ffast-math -o $(BUILD_DIR)/$(DSP_TARGET) Audioengine/StelleEngine/dsp/stelle_dsp.c

build: clean build-dsp
	cd $(WEBUI_DIR) && wails build $(WTAGS) -o $(APP_NAME)
	mv $(WEBUI_DIR)/build/bin/* $(BUILD_DIR)/

build-dev-win: clean build-dsp
	cd $(WEBUI_DIR) && wails build $(WTAGS) -debug -windowsconsole -o $(APP_NAME)
	mv $(WEBUI_DIR)/build/bin/* $(BUILD_DIR)/

.PHONY: default all build build-dev-win

.PHONY: clean
clean:
	rm -rf $(BUILD_DIR)