.PHONY: dev-up dev-setup dev-down dev-reset otel-up otel-down ent-generate help

generate:
	@echo "🔄 Generating SQLC code..."
	@cd configs && make reset
# 	@cd state && make reset
	@echo "✅ SQLC code generated"
# Show available commands
help:
	@echo "Available development commands:"
	@echo "  generate - Regenerate Ent code after schema changes"
	@echo "  help         - Show this help message"
