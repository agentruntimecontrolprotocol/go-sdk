# permission_challenge

Two-party challenge: a generator agent proposes a patch; a reviewer
agent holds the veto. The generator's `permission.request` carries
the patch fingerprint as part of the resource string, and the
reviewer's grant or deny is the only authoritative outcome.

## Before ARCP

A "code review bot" comments on a PR and the human merges; nothing
ties the merge gate to the agent's own runtime. The generator can
also re-trigger itself indefinitely after a deny — there's no
typed feedback channel.

## With ARCP

```go
reply, _ := generator.Request(ctx, &arcp.Envelope{
    IdempotencyKey: fmt.Sprintf("review:%s:%s", ticketID, fingerprint(patch.Diff)),
    Payload: &messages.PermissionRequest{
        Permission: "repo.write",
        Resource:   fmt.Sprintf("ticket:%s/%s", ticketID, fingerprint(patch.Diff)),
        Operation:  "apply_patch",
        Reason:     "apply patch",
        RequestedLeaseSeconds: 90,
    },
})
// reply is *messages.LeaseGranted or *messages.PermissionDeny.
// Identical patches dedupe at the runtime via idempotency_key.
```

The reviewer's deny is structured (`reason`) and feeds back into the
next generator turn as `priorDenial`.

## ARCP primitives

- Two-party permission challenge — RFC §15.4.
- `idempotency_key` for patch-level dedupe — §6.4.
- Resource scoping by content fingerprint.
- Typed deny → bounded retry loop.

## File tour

- `main.go` — generator drives a bounded revision loop; reviewer
  loop runs in a goroutine.
- `agents.go` — propose / review stubs + `Session` shim.

## Variations

- Replace the LLM reviewer with a static analyzer or OPA policy —
  the responder is interchangeable.
- Stream the patch as an artifact and reference it from the
  permission request (see [handoff](../handoff)).
- Add a third party: a security reviewer that holds an additional
  `repo.write.security` veto on touched files.
