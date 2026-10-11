# Preview reference GREEN

Complete remapping validation now rejects unresolved field-scoped option references before preflight success, without mutating the preview or allocating persistent/final IDs. Reuses V070 remapper with discarded temporary IDs; unchanged independent test and entire apptemplates race regression pass.
