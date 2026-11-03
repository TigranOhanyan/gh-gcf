# The label step calls GitHub with go-github

The label step uses `google/go-github` to get and create labels. The token is `GH_TOKEN`, then `GITHUB_TOKEN`. When both are empty, the step runs `gh auth token` and uses the printed token. A `gh` extension does not receive the saved login as `GH_TOKEN`, and the REST API accepts that token as a bearer token from this process. Tests point the client's base URL at WireMock and stub the get and the create. The token is not written into an event.

## Considered Options

- Shell out to `gh` for the label calls: rejected, because `gh`'s extra requests change between releases and the stubs would track that binary.
- Hand-written HTTP: rejected, because the later issue, pull request, and check calls belong on the same client.
