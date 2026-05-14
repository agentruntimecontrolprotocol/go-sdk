# lease-violation

Spec §9.3. The agent attempts `fs.write` outside the lease; the
runtime returns `PERMISSION_DENIED` surfaced as a tool_result body.
