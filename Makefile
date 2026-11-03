.PHONY: generate help test-up test-setup test-down test-run

generate:
	@echo "🔄 Generating SQLC code..."
	@cd configdb && make reset
	@cd statedb && make reset
	@echo "✅ SQLC code generated"

test-up:
	@echo "🚀 Starting test environment..."
	@docker compose -f ./.dev/docker-compose_test.yaml --env-file ./.dev/local.env -p gh-gcf up -d
	@echo "✅ Test environment started"

test-setup:
	@echo "🔧 Waiting for WireMock..."
	@. ./.dev/local.env && for i in $$(seq 1 60); do curl -fs http://127.0.0.1:$$WIREMOCK_EXPOSED_PORT/__admin/mappings >/dev/null && echo "✅ WireMock is ready" && exit 0; sleep 1; done; echo "WireMock did not become ready" >&2; exit 1

test-down:
	@echo "🛑 Stopping test environment..."
	@docker compose -f ./.dev/docker-compose_test.yaml --env-file ./.dev/local.env -p gh-gcf down
	@echo "✅ Docker containers stopped and removed"

test-run:
	@echo "🚀 Running tests..."
	go test ./... -count=1 -p 1
	@echo "✅ Tests finished"

help:
	@echo "Available development commands:"
	@echo "  test-up    - Start WireMock"
	@echo "  test-setup - Wait until WireMock answers"
	@echo "  test-down  - Stop and remove Docker containers"
	@echo "  test-run   - Run the tests"
	@echo "  generate   - Regenerate SQLC code"
	@echo "  help       - Show this help message"
