#!/usr/bin/env python3
"""Copy a live fixture with fresh resource UUIDs for a repeatable launch."""

import argparse
import json
from pathlib import Path
from uuid import uuid4


def fresh_ids(value):
    if isinstance(value, dict):
        return {
            key: str(uuid4()) if key in {"run_id", "suite_run_id"} else fresh_ids(item)
            for key, item in value.items()
        }
    if isinstance(value, list):
        return [fresh_ids(item) for item in value]
    return value


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("fixture", type=Path)
    parser.add_argument("--output", required=True, type=Path)
    args = parser.parse_args()
    value = fresh_ids(json.loads(args.fixture.read_text()))
    # Exclusive creation protects an earlier request from accidental overwrite.
    with args.output.open("x") as output:
        output.write(json.dumps(value, indent=2) + "\n")
    print(args.output)


if __name__ == "__main__":
    main()
