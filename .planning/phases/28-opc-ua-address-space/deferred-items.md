
## 28-04

- pkg/interp cannot initialise TOD, DATE or DT variables from literals (`runtime error: unsupported literal kind: Tod/Date/DateTime`). Found while testing pkg/opcua/bind; out of scope for the OPC UA adapters.
