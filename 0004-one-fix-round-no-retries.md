# One fix round, no retries

After both verdicts and CI are in, a fresh fixer runs once if any reviewer requested changes or CI is red, and the factory then stops for that pull request. Every error, including failed launches, marks the issue for attention instead of retrying. The factory starts with as few assumptions as possible and grows from what we observe.