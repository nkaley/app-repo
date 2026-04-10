#!/usr/bin/env python3
from pathlib import Path
import re
import sys


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: update_ops_image_tag.py <values_file> <new_tag>", file=sys.stderr)
        return 2

    path = Path(sys.argv[1])
    new_tag = sys.argv[2]

    text = path.read_text()

    pattern = re.compile(
        r"(^image:\n(?:[ \t]+.*\n)*?[ \t]+tag:[ \t]*)[^\n]+",
        flags=re.MULTILINE,
    )
    new_text = pattern.sub(rf"\1{new_tag}", text, count=1)

    path.write_text(new_text)
    print(f"Updated {path} to tag {new_tag}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
