# Datastar in this repo

For Datastar itself, use the official pages (they have the examples):

- [Getting started](https://data-star.dev/guide/getting_started)
- [Attributes](https://data-star.dev/reference/attributes)
- [Actions](https://data-star.dev/reference/actions)
- [SSE](https://data-star.dev/reference/sse_events)
- [Examples](https://data-star.dev/examples)
- [Go SDK GitHub](https://github.com/starfederation/datastar-go)


What is ours: `knowledge/webui` (scraper) and `chat/` (lobby). Script is Datastar `v1.0.2`. Go SDK package name is `github.com/starfederation/datastar-go/datastar`. Handlers use `ReadSignals`, `NewSSE`, `PatchElementTempl`, `MarshalAndPatchSignals`. Pages mount under a stripped prefix (`/scraper/`, `/app/`) so actions stay relative (`@post('chat')`).
