BINARY=content
BUILD_DIR=./bin

.PHONY: build run-cron run-cron-now run-plan run-daily run-sync-avatars run-check-videos run-check-approval run-status run-compose run-edit clean tidy

build:
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/content

tidy:
	go mod tidy

run-cron: build
	$(BUILD_DIR)/$(BINARY) cron

run-cron-now: build
	$(BUILD_DIR)/$(BINARY) cron --now

run-plan: build
	$(BUILD_DIR)/$(BINARY) plan

run-daily: build
	$(BUILD_DIR)/$(BINARY) daily

run-sync-avatars: build
	$(BUILD_DIR)/$(BINARY) sync-avatars

run-check-videos: build
	$(BUILD_DIR)/$(BINARY) check-videos

run-check-approval: build
	$(BUILD_DIR)/$(BINARY) check-approval

run-status: build
	$(BUILD_DIR)/$(BINARY) status

run-compose: build
	$(BUILD_DIR)/$(BINARY) compose $(ID)

run-edit: build
	$(BUILD_DIR)/$(BINARY) edit $(ID)

clean:
	rm -rf $(BUILD_DIR)
