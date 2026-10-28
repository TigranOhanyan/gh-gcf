# Each reviewer posts its own verdict as its own GitHub App

Cloud agents act as the user, and GitHub does not let a user approve or request changes on their own pull request. Each reviewer role gets its own App and receives a short-lived installation token as `GH_TOKEN`, so its review counts as a separate approval. The App's private key never reaches an agent.