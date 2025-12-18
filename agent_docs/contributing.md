# Contributing

## Requirements

- All changes require a referenced [PROJQUAY Jira](https://issues.redhat.com/projects/PROJQUAY/issues)
- Fork the repository and create a topic branch from `master`

## Commit Message Format

```
<subsystem>: <what changed> (PROJQUAY-####)

<why this change was made>
```

Example:
```
controllers: fix namespace finalizer race condition (PROJQUAY-1234)

The finalizer was being removed before cleanup completed,
causing orphaned Quay organizations.
```

Guidelines:
- Subject line: max 70 characters
- Body: wrap at 80 characters
- Explain the "why", not just the "what"

## Contact

- Issues: [PROJQUAY Jira](https://issues.redhat.com/projects/PROJQUAY/issues)
- Email: [quay-sig@googlegroups.com](https://groups.google.com/g/quay-sig)
- IRC: #quay on freenode.org
