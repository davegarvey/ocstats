BIN := ocstats
PREFIX ?= $(HOME)/.local
INSTALL_DIR := $(PREFIX)/bin
# SwiftBar plugins dir: https://github.com/swiftbar/SwiftBar (free menu bar host)
PLUGIN_DIR := $(HOME)/Library/Application Support/SwiftBar/plugins
PLUGIN := scripts/ocstats.swiftbar.sh
PLUGIN_TARGET := $(PLUGIN_DIR)/ocstats.15m.sh

.PHONY: build test vet fmt install plugin

build:
	go build -o $(BIN) .

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -l -w .

plugin: $(PLUGIN_TARGET)

$(PLUGIN_TARGET): $(PLUGIN)
	install -d "$(PLUGIN_DIR)"
	install -m 0755 $(PLUGIN) "$(PLUGIN_TARGET)"

install: build plugin
	install -d "$(INSTALL_DIR)"
	install -m 0755 $(BIN) "$(INSTALL_DIR)/$(BIN)"
	@echo "Installed $(BIN) to $(INSTALL_DIR)/$(BIN)"
	@echo "Installed SwiftBar plugin to $(PLUGIN_TARGET)"
	@echo "Restart SwiftBar (or run: killall SwiftBar) to pick up the plugin."
