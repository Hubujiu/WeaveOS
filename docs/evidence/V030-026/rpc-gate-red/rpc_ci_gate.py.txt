"""Compile-only Root declaration for the additive RPC report gate."""
class GateError(Exception):
    pass
def validate(report_dir, manifest, go_json_path):
    raise GateError("RPC report gate not implemented")
