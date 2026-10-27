# Examples

Take examples from this repository.

## Accumulative testing

[knowledge/embedder/const_test.go](../../../knowledge/embedder/const_test.go) is `package embedder_test`, so it can import the embedder and the earlier packages without an import cycle. `TestMain` wires the live ingestor, scraper, uploader, and embedder.

[knowledge/embedder/embedder_test.go](../../../knowledge/embedder/embedder_test.go) erases the tables, then calls `Ingest`, `Scrape`, and `Upload` before `Embed`. The assertion counts the WireMock embedding requests.

## WireMock and local Postgres

[embeddings/embedding_cache_test.go](../../../embeddings/embedding_cache_test.go) stubs OpenAI with WireMock and reads the cached row back from local Postgres.

## Error paths

A refusal asserts the error, then asserts that nothing was written. The example is complete here.

```go
func Test_InsertTask_should_refuse_a_number_that_is_already_stored(t *testing.T) {
	eraseCrawlJobs(t)
	job := createJob(t)

	_, err := testExpandWorker.InsertTask(t.Context(), job.ID, 60503)
	require.NoError(t, err)

	task, err := testExpandWorker.InsertTask(t.Context(), job.ID, 60503)
	require.Error(t, err)
	assert.Zero(t, task)
	assert.Equal(t, 1, countTasks(t, job.ID))
}
```

The first `InsertTask` builds the state through production code. The second is the call under test. `countTasks` counts the job's rows in `crawl_tasks`, so `1` proves the refused call stored nothing.
