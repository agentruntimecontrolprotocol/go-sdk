# lease-expires-at

Spec §9.5. Submit with `lease_constraints.expires_at = now + 2s`. The
agent loops past the deadline; the runtime watchdog fires
`LEASE_EXPIRED`.
