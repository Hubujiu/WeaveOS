"""Root declaration: execution RPC report validation not implemented yet."""
import sys
class GateError(Exception):
 pass
def validate(report_dir, manifest, unit_json, interop_json):
 raise GateError("execution RPC report gate unimplemented")
if __name__ == '__main__':
 sys.exit(1)
