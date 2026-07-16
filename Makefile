APP := switchboard
HOME_DIR := $(LOCALAPPDATA)/Switchboard

.PHONY: build test icon install uninstall ui doctor clean

build:
	go build -ldflags "-H windowsgui -s -w" -o bin/$(APP).exe .

test:
	go test ./...

# regenerate assets/icon.ico and the .syso resource that go build embeds
icon:
	go run ./tools/genicon
	go run github.com/akavel/rsrc@latest -ico assets/icon.ico -o rsrc_windows_amd64.syso

# copy the exe to its stable home and register that copy, so rebuilding or
# moving the repo never breaks the browser registration
install: build
	cp bin/$(APP).exe "$(HOME_DIR)/$(APP).exe"
	"$(HOME_DIR)/$(APP).exe" install

uninstall:
	./bin/$(APP).exe uninstall

ui: build
	./bin/$(APP).exe ui

doctor: build
	./bin/$(APP).exe doctor

clean:
	rm -rf bin
