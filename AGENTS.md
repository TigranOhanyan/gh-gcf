# Agent instructions

## Change discipline

- Propose the change first. Wait for agreement. Then edit.
- One step at a time. Do not bundle unrelated edits.
- Do not bulldoze existing code. Do not refactor what the step does not require.
- Keep the file's style and conventions.

When writing, editing, or reviewing Go: follow [CODING_STANDARDS.md](CODING_STANDARDS.md).

## Agent skills

### Issue tracker

Issues and specs live in this repo's GitHub Issues, via the `gh` CLI. See `docs/agents/issue-tracker.md`.

### Domain docs

Single-context: one `GLOSSARY.md` and `docs/adr/` at the repo root. See `docs/agents/domain.md`.

## Cursor Cloud specific instructions

The cloud machine is set up by `.cursor/environment.json`, `.cursor/Dockerfile` and `.cursor/start.sh`.

- Go, Docker, the AWS CLI and `samlocal` are installed. Dependencies are downloaded and the code is compiled during the build.
- At start, `.cursor/start.sh` runs `make test-up` and `make test-setup`, so Postgres, LocalStack, WireMock and Cognito are already running. If they are down, run `bash .cursor/start.sh` again.
- Tests read the root `.env`. It holds only local and fake values. Real keys are not available in the cloud, and tests must not call real external services; use WireMock.
- Follow the testing skill in `.claude/skills/testing/SKILL.md`. Always run tests with `-p 1`, and finish with `make test-run`.
