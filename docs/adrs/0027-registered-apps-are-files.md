# Registered Apps are files named by App ID

The factory keeps one empty file per App at `apps/<app id>` on main. The file name is the App ID. Setup creates the file, and a repo's config then stores that same id for the blueprints that use it. Bind reads the directory and writes one `GITHUB_APP_<app id>` line into the tick workflow, taken from the Actions secret of that name. The private key stays in the secret. The `app` step reads that environment variable and does not read the file. The factory app keeps `FACTORY_GITHUB_APP_ID` and `FACTORY_GITHUB_APP_PRIVATE_KEY`.

## Considered Options

- A third SQLite file for the registry: rejected, because the registry is a set of integers, and a binary file hides an added id in the diff.
- Collecting the ids by opening every repo's config: rejected, because an App is registered before any repo uses it, and bind would open every database to find them.
- A second workflow that lists the secrets: rejected, because bind already rewrites the tick workflow, and the second job would check out the repo again.
