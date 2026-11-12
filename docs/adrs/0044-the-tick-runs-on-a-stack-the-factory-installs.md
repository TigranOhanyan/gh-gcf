# The tick runs on a stack the factory installs

GitHub Actions could not keep a reliable schedule, and it is not this factory's compute. Bind writes an install the user runs by hand. That install deploys one stack into an AWS account the factory already uses. The stack is the tick. Several factories may share the account. The stack name, an Actions variable on the factory repo, is what keeps them apart. Deleting the factory repo leaves the tick running. Deleting the stack is the uninstall. This supersedes the uninstall sentence in `0001-separate-factory-repo.md`.

## Considered Options

- One stack owned by gh-gcf for every factory: rejected, because the factory repo would no longer be the install, and one project would hold every factory's keys.
- A second AWS account per factory: rejected, because the account is already shared and the stack name separates factories.
- Deleting the factory repo as the uninstall: rejected, because the tick would keep running after the repo was gone.
