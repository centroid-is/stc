# Deferred Items: Phase 24

- (24-02) `go test ./pkg/checker` fails `TestEmptyFBCall` (SEMA022 "fb is not callable") on this branch. Pre-existing: pkg/checker does not depend on pkg/ecat or any file changed by 24-02. Out of scope for phase 24.
