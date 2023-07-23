 # protoc --go_out=. --go_opt=module=proto ./pb/crdt-broadcast.proto
# Variables
PROTOC := protoc
PROTO_DIR := ./pb
PROTO_FILES := $(PROTO_DIR)/*.proto
GO_OUT := .
GO_OPT := module=proto

# Targets
.PHONY: all clean

all: generate

generate:
	$(PROTOC) --go_out=$(GO_OUT) --go_opt=$(GO_OPT) $(PROTO_FILES)

clean:
	rm -rf $(GO_OUT)/*.go
