# Coding standards

## Go

Named result parameters and naked `return`.

Never construct a value in a call argument. Bind it first. See Exceptions.

Uncle Bob: name intermediate results. Do not nest work in the call.

Never put business logic in `if` / `switch` / `for` conditions. Compute, assign, then branch (ok pattern).

Never edit think comments in code.

The admin database only creates the other databases, or ensures they exist. After `NewAdminDatabase`, close it. Pass the pointer to the other database constructors anyway. Those constructors take `_ *divanbashi.AdminDatabase` and do not use it. The unused parameter makes the creation order a compile-time requirement: nobody can open a domain database without first going through admin. [db.go](db.go) and [knowledge/db.go](knowledge/db.go) are the example.

### Exceptions

These may be passed straight into a call. Each one may also be bound to a variable first. The list is a holding pen until the pattern among them becomes a rule.

- A type conversion, such as `string(status)` or `int32(1)`.
- A zap field constructor: `zap.Error`, `zap.String`, and the other `zap` field helpers.
- `t.Context()`.

### Examples

Bind, then branch. Do not declare inside the condition:

```go
// bad
if n, err := strconv.Atoi(s); err != nil {
	return err
}

// good
n, err := strconv.Atoi(s)
if err != nil {
	return err
}
```

Bind call arguments before the call. Do not construct values in the argument list:

```go
// bad
send(Chunk{EventHeader: NewEventHeader(uid, cid, sid, iid), Delta: d})

// good
header := NewEventHeader(uid, cid, sid, iid)
event := Chunk{EventHeader: header, Delta: d}
send(event)

// A type conversion may stay in the argument list.
assert.Equal(t, int32(1), row.Number)
assert.Equal(t, string(status), row.Status)

// Zap field constructors may stay in the argument list.
zap.L().Error("ensure admin reader", zap.Error(err))
```

Close the admin database after it has created the cluster databases. Pass the closed pointer so later constructors still require the admin step:

```go
adminDB, err := divanbashi.NewAdminDatabase(ctx, adminURL)
if err != nil {
	return err
}
adminDB.Close()

knowledgeDB, err := knowledge.NewKnowledgeDatabase(ctx, knowledgeURL, adminDB)
if err != nil {
	return err
}
```

## Tests

Tests follow the `testing` skill: [.claude/skills/testing/SKILL.md](.claude/skills/testing/SKILL.md).
