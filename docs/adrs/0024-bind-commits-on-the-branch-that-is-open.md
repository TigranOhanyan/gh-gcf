# Bind commits on the branch that is open

Bind uses the current directory as the factory repo and commits the workflow on the branch checked out there. It does not push. A factory repo that refuses a direct change to main reviews that commit in a pull request. The tick still reads main.

## Considered Options

- A factory path flag: rejected, because the user runs bind in the checkout they mean to change.
- Committing only on main: rejected, because the change is reviewed before it reaches main.
