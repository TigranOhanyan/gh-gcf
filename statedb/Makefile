.PHONY: generate clean help

# Generate sqlc code from queries and migrations
generate:
	@echo "🔄 Generating sqlc code..."
	@go tool sqlc generate
	@echo "✅ Generation complete!"

# Clean generated files
clean:
	@echo "🧹 Cleaning sqlc generated files..."
	@rm -rf ./entities/ 2>/dev/null || true
	@echo "✅ Cleanup complete! (removed entities/ directory)"

# Clean and regenerate
reset: clean generate


# Show help
help:
	@echo "Available targets:"
	@echo "  generate    - Generate sqlc code from schemas"
	@echo "  clean       - Remove all generated files"
	@echo "  reset       - Clean and regenerate (fresh start)"
	@echo "  help        - Show this help message"

