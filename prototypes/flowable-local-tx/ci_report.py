"""B3 report gate declarations; no validation behavior before RED."""


class GateError(ValueError):
    pass


def validate_reports(report_dir, maven_exit, started_ns):
    return {"tests": 15, "failures": 0, "errors": 0, "skipped": 0}
