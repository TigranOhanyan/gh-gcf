# A separate factory repo drives app repos

All factory config, state, workflows and secrets live in a factory repo that acts on app repos through GitHub App tokens, so app repos carry no factory files and binding or deleting the factory repo is the whole install and uninstall. This rules out per-repo hooks in app repos, which is why agent behaviour is set by prompt and launch settings only.