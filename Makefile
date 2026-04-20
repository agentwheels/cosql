BIN        := cosql
PREFIX     ?= /usr/local
SKILL_DIR  ?= $(HOME)/.claude/skills/cosql
CONFIG_DIR ?= $(HOME)/.config/cosql

.PHONY: all build install install-skill install-example-config uninstall test tidy clean

all: build

build:
	mkdir -p bin
	go build -o bin/$(BIN) ./cmd/cosql

install: build install-skill
	install -m 0755 bin/$(BIN) $(PREFIX)/bin/$(BIN)

install-skill:
	mkdir -p $(SKILL_DIR)/references
	cp skills/cosql/SKILL.md $(SKILL_DIR)/SKILL.md
	cp skills/cosql/references/*.md $(SKILL_DIR)/references/

install-example-config:
	mkdir -p $(CONFIG_DIR)
	@if [ -f $(CONFIG_DIR)/config.toml ]; then \
		echo "$(CONFIG_DIR)/config.toml already exists, skipping"; \
	else \
		install -m 0600 examples/config.toml $(CONFIG_DIR)/config.toml; \
		echo "installed example config to $(CONFIG_DIR)/config.toml"; \
	fi

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)
	rm -rf $(SKILL_DIR)

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin/
