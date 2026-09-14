#!/usr/bin/env python3
"""Validate machine-readable FlyQPro transfer evidence.

The validator is deliberately conservative: a report may be pass, fail, or
blocked, but never silently omit its status or sample outcome.
"""

from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path


ALLOWED = {"pass", "fail", "blocked"}


def validate(path: Path) -> list[str]:
    errors: list[str] = []
    try:
        document = json.loads(path.read_text(encoding="utf-8"))
    except Exception as exc:  # pragma: no cover - command-line failure path
        return [f"{path}: invalid JSON: {exc}"]
    if not isinstance(document, dict):
        return [f"{path}: report must be an object"]

    status = document.get("status")
    if status not in ALLOWED:
        errors.append(f"{path}: status must be one of {sorted(ALLOWED)}")
    if document.get("schemaVersion") != 1:
        errors.append(f"{path}: schemaVersion must be 1")
    for key in ("buildId", "commit", "worktree", "transport", "network"):
        if not isinstance(document.get(key), str) or not document[key].strip():
            errors.append(f"{path}: missing non-empty {key}")

    samples = document.get("samples")
    checks = document.get("checks")
    if samples is None and checks is None:
        errors.append(f"{path}: report must contain samples or checks")
        return errors
    if samples is None:
        samples = []
    if not isinstance(samples, list):
        errors.append(f"{path}: samples must be an array")
        samples = []
    for index, sample in enumerate(samples):
        prefix = f"{path}: samples[{index}]"
        if not isinstance(sample, dict):
            errors.append(f"{prefix} must be an object")
            continue
        result = sample.get("result")
        if result not in ALLOWED:
            errors.append(f"{prefix}.result must be one of {sorted(ALLOWED)}")
        for key in ("scenario", "canonicalState", "sessionId"):
            if not isinstance(sample.get(key), str) or not sample[key].strip():
                errors.append(f"{prefix}: missing non-empty {key}")
        for key in ("bytes", "finalFileBytes", "durableBytes"):
            if not isinstance(sample.get(key), int) or sample[key] < 0:
                errors.append(f"{prefix}.{key} must be a non-negative integer")
        if not isinstance(sample.get("sha256Verified"), bool):
            errors.append(f"{prefix}.sha256Verified must be boolean")
        if result == "pass":
            if sample.get("canonicalState") != "completed":
                errors.append(f"{prefix}: pass requires canonicalState=completed")
            if sample.get("sha256Verified") is not True:
                errors.append(f"{prefix}: pass requires sha256Verified=true")
            if sample.get("finalFileBytes") != sample.get("bytes"):
                errors.append(f"{prefix}: pass requires finalFileBytes=bytes")
            if sample.get("durableBytes") != sample.get("bytes"):
                errors.append(f"{prefix}: pass requires durableBytes=bytes")
    if checks is not None:
        if not isinstance(checks, list) or not checks:
            errors.append(f"{path}: checks must be a non-empty array")
        else:
            for index, check in enumerate(checks):
                prefix = f"{path}: checks[{index}]"
                if not isinstance(check, dict) or check.get("status") not in ALLOWED:
                    errors.append(f"{prefix}.status must be one of {sorted(ALLOWED)}")
                elif not isinstance(check.get("name"), str) or not check["name"].strip():
                    errors.append(f"{prefix}.name must be non-empty")
    return errors


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("reports", nargs="+", type=Path)
    args = parser.parse_args()
    errors = [error for report in args.reports for error in validate(report)]
    if errors:
        print("\n".join(errors), file=sys.stderr)
        return 1
    for report in args.reports:
        print(f"validated: {report}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
