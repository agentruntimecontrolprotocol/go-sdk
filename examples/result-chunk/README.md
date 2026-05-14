# result-chunk

Spec §8.4. The agent streams a 5 KB result via `JobContext.StreamResult`;
the client reassembles using `JobHandle.CollectChunks`.
