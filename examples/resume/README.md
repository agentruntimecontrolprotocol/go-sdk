# resume

Spec §6.3. Resume token rotation and event replay. The current
in-memory event log retains envelopes per session; replay is wired
into the runtime via `session.hello.payload.resume`.

This example is a stub; the resume flow is exercised by the
conformance harness.
