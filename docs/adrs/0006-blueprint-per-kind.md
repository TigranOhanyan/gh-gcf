# Builders, reviewers, and fixers are separate blueprints

The building cycle is not a generic cycle graph. Config has one table per blueprint kind. A builder is tied to a starting label. Reviewers and fixers are tied to a builder by name, so two builders can share a GitHub App and remain independent flows. Each kind discovers its own work in code. State keeps a run for each blueprint working on one issue.

## Considered Options

- One generic cycle table: rejected, because each flow discovers its issues with its own query, and only the building flow is defined.
- A table per named blueprint: rejected, because adding a blueprint would be a schema migration.
