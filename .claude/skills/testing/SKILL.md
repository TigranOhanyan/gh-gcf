---
name: testing
description: >-
  Go test style: one behavior per test, assert stored state,
  accumulative setup, WireMock for HTTP, real local Postgres, and a
  per-test git repo for the factory's main and state branches.
  Use when writing, editing, or reviewing tests, *_test.go files, or
  when the user mentions accumulative testing, WireMock, or test setup.
---

# Testing

Also follow [CODING_STANDARDS.md](../../../CODING_STANDARDS.md). Tests are Go.

## Shape

One function, one outcome. Name it `Test_<Subject>_should_<outcome>`.

Repeat setup across functions. Each function asserts one slice of the result.

Assert the outcome the name promises. Check the full shape of a result (every field of the returned value, every column of the stored row) once for each code path that produces it, in its own test. Other tests assert only their outcome.

When a call is expected to return a nil error, confirm that with `require.NoError`, so the test stops at the first unexpected error instead of running on bad state. Do this for setup (an earlier production step, a WireMock stub, reading a fixture) and for the call under test. When a call is expected to return an error, confirm that with `require.Error`. Do not write `if err != nil { t.Fatal(err) }` by hand.

Use `require` also for a precondition the next line depends on: `require.Len` or `require.NotEmpty` before indexing into a result.

Use `assert` for checks on the result, so one failed check does not hide the ones after it.

Also read what was stored: the database, object storage, the graph, or a WireMock request count.

Do not mock the production code that prepares data.

Define a function that exists only to serve tests in the test file. Leave it in production code when generated code must call it, or when its types are not visible from the test package.

Wire every dependency the tests use in `TestMain`, in `const_test.go`: databases, clients, and the production components, including earlier steps and the subject under test. [knowledge/embedder/const_test.go](../../../knowledge/embedder/const_test.go) is the example. It wires Postgres, S3, the OpenAI client, WireMock, and the live ingestor, scraper, uploader, and embedder. `TestMain` resolves `testdata` next to the package, and `testdata/junk` inside it.

Checked-in fixtures live in that `testdata` folder. [embeddings/testdata](../../../embeddings/testdata) is the example.

When the code writes files, point them at `testdata/junk`. Put a `_keepme` file in `junk` when you create the folder, so Git tracks the empty folder. Every other file in `junk` stays out of Git. See [.gitignore](../../../.gitignore).

Give each run a fresh id. Bound timestamps with `startOfTest` one second before the call and `startOfCheck` one second after, then `assert.WithinRange`.

On an error path, also assert the write did not happen.

When a test has `t`, use `t.Context()`. Do not use `context.TODO` or `context.Background` there. [embeddings/embedding_cache_test.go](../../../embeddings/embedding_cache_test.go) is the example. `TestMain` has no `t`, so it uses `context.Background`.

## Services

Postgres is a real database running locally. Mock HTTP with WireMock (`github.com/wiremock/go-wiremock`). Do not use a mock HTTP client from the Go standard library.

[embedding_cache_test.go](../../../embeddings/embedding_cache_test.go) is the representative test. WireMock stands in for OpenAI, and the test reads the cached row back from local Postgres.

## Isolation

Parallel tests are forbidden. This is a hard requirement. Tests share the databases, the S3 buckets, the WireMock instance, and the data in them, so two tests running at once corrupt each other's state.

- Always run tests with `-p 1`, so one package runs at a time. `make test-run` already pass it. When you run `go test` yourself, on one package or a subset, pass `-p 1` too.
- Never call `t.Parallel()`, in a test or in a subtest.

When the query scans the whole table, erase the database before the test so it starts empty.

When the logic is bound to a tenant, create a unique random tenant during setup. That tenant is the boundary that keeps this test off another test's rows.

## Running tests

From the repo root, in this order. The targets are in the [Makefile](../../../Makefile).

1. `make test-up` starts the test environment.
2. `make test-setup` sets up that environment: migrations, S3 buckets, and other resources.
3. `make generate` regenerates source when it is needed (sqlc, templ, and the rest).
4. `make test-run` run the tests.

`make test-up` and `make test-setup` are idempotent. Run both every time before the tests, including when the containers are already up.

For a quick check of the logic in one package, run `go test` on that package, and narrow it with `-run` to one test or a pattern:

```sh
go test ./knowledge/embedder/ -run 'Test_Embedder_should_request_an_embedding_for_each_legal_text' -v -p 1 -count=1
go test ./knowledge/scraper/ -run 'Test_IrtecScraper_.*' -v -p 1 -count=1
go test ./knowledge/... -v -p 1 -count=1
```

`make test-run` runs every test in the repo, one package at a time (`-p 1`). Use it for the final run, to confirm everything works before you finish.

## Accumulative testing

This project always uses accumulative testing for setup.

When a test needs data that an earlier step produces, call that earlier production code in the test environment and let it build the data. Do not write a second setup path that inserts the same rows or files by hand.

If a later test fails, read the results as a cascade: if every preceding flow is green, the failure is in the flow under test.

A cycle can make that setup impossible inside the production package. Keep the test files next to the code, and declare them `package foo_test`, so the test can import the package under test and the earlier packages.

The example is [knowledge/embedder/embedder_test.go](../../../knowledge/embedder/embedder_test.go), wired in [knowledge/embedder/const_test.go](../../../knowledge/embedder/const_test.go).

The argument for the approach is in [accumulative_testing.md](accumulative_testing.md). Read that for the why, not for how to write a test here. The essay's recipe is for a team that uses accumulative testing only sometimes. Tests in this repository follow the always-accumulative preference.

Take examples from this repository. Do not copy tests from another codebase. A short index is in [examples.md](examples.md).

## Factory branches

A test that reads `config.db` from `main` and `state.db` from `state` builds that repository in its setup, under `testdata/junk/<test name>`. See [0016-worktrees-for-config-and-state.md](../../../docs/adrs/0016-worktrees-for-config-and-state.md).

1. `git init -b main`.
2. Create `repos/<owner>/<name>/config.db` with `configdb.Open` and this test's rows, then commit it.
3. `git checkout -b state`.
4. Create `state.db` there with `statedb.Open` and this test's rows, then commit it.
5. Call the code under test on that folder.

Each test gets its own `main` and `state`. Do not check in a factory repository. The databases differ per test.
