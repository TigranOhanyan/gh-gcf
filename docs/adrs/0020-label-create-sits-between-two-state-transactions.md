# A label create sits between two state transactions

The label step commits the new state row and the starting event, calls GitHub outside any transaction, then commits the result event. The git commit of that repo's state happens once after the step returns, including when the call failed or nothing in the file changed. A failed create leaves the row with no creation time. The next run sees the row, creates the missing label, and records the creation time. A label that was already on GitHub is recorded with no creation time and no event.

## Considered Options

- One transaction wrapped around the GitHub call: rejected, because the call would be held inside the transaction.
