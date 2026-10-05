"""Strict two-suite execution report gate; Root declaration before implementation."""
class GateError(Exception):
    pass

def validate(report_dir, manifest):
    raise GateError("not implemented")

if __name__ == "__main__":
    raise SystemExit("execution report gate is not implemented")
