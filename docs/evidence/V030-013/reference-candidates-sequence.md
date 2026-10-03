# Actual reference candidate implementation sequence

The first candidate fixture attempted a duplicate persisted grant tuple, so its
SQL23505 failure is NOT a target RED. A first implementation was attempted before
that output was checked. That attempt was reverted completely; corrected fixture
uses an independent real permission group rather than weakening unique grant
constraints. This is an ordering mistake, not presented as a pristine TDD run.

The restored source then produced the genuine missing-route behavior: field/action
permitted ordinary candidate GET returned400, expecting200. Its corrected fixture
and absent-route source are retained separately in reference-field-http-target-red
and source.tar. Only then was the candidate implementation reapplied. The original
invalid fixture/log and initial source archive remain separately recoverable.
