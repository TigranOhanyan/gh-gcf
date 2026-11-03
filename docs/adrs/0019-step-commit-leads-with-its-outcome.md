# A step commit leads with its outcome

A step's state commit starts with ✅ or ❌, then the tick id, the step name, and the repo. ✅ means the step returned no error. ❌ means it did. The body holds the step's message and, when both are set, the error text after it. Each body line starts with ✅ or ❌ so a review of the state history can be scanned. The error text is kept as the call returned it. The tick command's own commit stays `[tick id] [tick] [repo] created`.

## Considered Options

- Put the message in the subject: rejected, because a step can create one label and fail on the next, and both results belong in the body.
- Put the mark after the repo: rejected, because the outcome is what a review scans for first.
