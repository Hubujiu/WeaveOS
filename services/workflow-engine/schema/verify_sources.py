"""Read the pinned native PostgreSQL sources without executing their SQL."""

import hashlib
import os
from pathlib import Path
import stat


_INVALID_BUNDLE = "invalid Flowable native SQL bundle"
_SOURCES = (
    (
        "flowable.postgres.create.common.sql",
        "9854e360694ab3a25ae417e7440a94194bad33cf9f193a92f363f21bfcb919bc",
        20129,
    ),
    (
        "flowable.postgres.create.engine.sql",
        "48fdd12cb080ab4dd6db9abef4e2b5c2f4b34d923feeca70d7db44b645f07e9f",
        11934,
    ),
    (
        "flowable.postgres.create.history.sql",
        "62bd2fe7c2a8583aa6638e39608c49360bc3ffdb1bd9ad3511683757f2b75bab",
        3919,
    ),
)


def load_verified_native_sql(root: Path, version: str = "8.0.0") -> tuple[str, ...]:
    """Return the exact common, engine and history SQL, or a safe ValueError.

    Hashes are fixed here rather than trusted from a caller-controlled manifest.
    Descriptor-relative, no-follow reads reject symlinks and avoid reopening a
    checked pathname. Reads are bounded to the original resources' byte sizes.
    """
    if version != "8.0.0":
        raise ValueError(_INVALID_BUNDLE)

    try:
        directory = os.open(root, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            verified = []
            for name, expected_hash, expected_size in _SOURCES:
                descriptor = os.open(
                    name,
                    os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK,
                    dir_fd=directory,
                )
                with os.fdopen(descriptor, "rb") as source:
                    metadata = os.fstat(source.fileno())
                    if not stat.S_ISREG(metadata.st_mode) or metadata.st_size != expected_size:
                        raise ValueError(_INVALID_BUNDLE)
                    raw = source.read(expected_size + 1)
                if len(raw) != expected_size or hashlib.sha256(raw).hexdigest() != expected_hash:
                    raise ValueError(_INVALID_BUNDLE)
                verified.append(raw.decode("utf-8", errors="strict"))
            return tuple(verified)
        finally:
            os.close(directory)
    except (OSError, TypeError, UnicodeError):
        raise ValueError(_INVALID_BUNDLE) from None
