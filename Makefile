# Vantage Security Hub — Ultimate Makefile
# (c) 2026 Vantage Principal Auditor

BINARY=vantage
GO=go
TOOL_DIR=./tools
OS=$(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(shell uname -m)

ifeq ($(ARCH),x86_64)
	ARCH=amd64
endif

PD_TOOLS = subfinder naabu httpx nuclei katana dnsx tlsx uncover asnmap cloudlist assetfinder interactsh-client
OTHER_TOOLS = chisel

.PHONY: all build build-tools capabilities install clean

all: build build-tools capabilities

build:
	@echo "[*] Building Vantage binary..."
	$(GO) build -v -o $(BINARY) gophish.go

build-tools:
	@mkdir -p $(TOOL_DIR)
	@echo "[*] Downloading ProjectDiscovery Suite..."
	@$(foreach tool,$(PD_TOOLS), \
		echo "[+] Installing $(tool)..."; \
		GOBIN=$(abspath $(TOOL_DIR)) $(GO) install github.com/projectdiscovery/$(tool)/v2/cmd/$(tool)@latest; \
	)
	@echo "[+] Downloading Chisel..."
	@GOBIN=$(abspath $(TOOL_DIR)) $(GO) install github.com/jpillora/chisel@latest

capabilities:
	@echo "[*] Granting network capabilities to naabu..."
	@sudo setcap cap_net_raw=eip $(TOOL_DIR)/naabu || echo "[!] WARNING: Could not set capabilities. Run manually: sudo setcap cap_net_raw=eip $(TOOL_DIR)/naabu"

install: build build-tools capabilities
	@echo "[*] Vantage installation complete. Tools are in $(TOOL_DIR)"

clean:
	rm -f $(BINARY)
	rm -rf $(TOOL_DIR)
