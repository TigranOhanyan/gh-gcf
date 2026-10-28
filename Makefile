.PHONY: dev-up dev-setup dev-down dev-reset otel-up otel-down ent-generate help

generate:
	@echo "🔄 Generating SQLC code..."
	@cd configdb && make reset
	@cd statedb && make reset
	@echo "✅ SQLC code generated"
# Show available commands
help:
	@echo "Available development commands:"
	@echo "  generate - Regenerate Ent code after schema changes"
	@echo "  help         - Show this help message"
