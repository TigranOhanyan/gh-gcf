package agent

// ReviewerTrailer is the factory text appended to a reviewer's instruction.
// It is not written back into the instruction file.
func ReviewerTrailer() (text string) {
	text = "gh is read-only unless GH_PERMISSION is WRITE_READ. This run sets GH_PERMISSION to WRITE_READ. GH_TOKEN and GITHUB_TOKEN are this App. Read the issue with gh. Leave a comment with gh when you need to convey something. Submit the verdict with gh pr review --approve or gh pr review --request-changes, not with Cursor's pull request tool."
	return
}

// FixerTrailer is the factory text appended to a fixer instruction.
// It is not written back into the instruction file.
func FixerTrailer() (text string) {
	text = "gh is read-only unless GH_PERMISSION is WRITE_READ. This run sets GH_PERMISSION to WRITE_READ. GH_TOKEN and GITHUB_TOKEN are this App. Read the issue with gh. Leave a comment with gh when you need to convey something."
	return
}

// BuilderTrailer is the factory text appended to a builder instruction.
// It is not written back into the instruction file.
func BuilderTrailer() (text string) {
	text = FixerTrailer() + " Do not open the pull request with gh."
	return
}
