#!/usr/bin/env python3
"""Lightweight repository secret scan for release checks."""

from __future__ import annotations

import re
import sys
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]

SKIP_DIRS = {
    ".git",
    ".playwright-cli",
    "dist",
    "node_modules",
    "__pycache__",
}

TEXT_SUFFIXES = {
    ".go",
    ".ts",
    ".tsx",
    ".vue",
    ".js",
    ".mjs",
    ".cjs",
    ".json",
    ".yaml",
    ".yml",
    ".toml",
    ".env",
    ".example",
    ".md",
    ".sh",
    ".service",
    ".sql",
    ".txt",
}

SECRET_PATTERNS = [
    ("openai_project_key", re.compile(r"\bsk-proj-[A-Za-z0-9_-]{80,}\b")),
    ("openai_api_key", re.compile(r"\bsk-(?!test-|usage-|getby|update-|reuse-)[A-Za-z0-9_-]{40,}\b")),
    ("anthropic_api_key", re.compile(r"\bsk-ant-[A-Za-z0-9_-]{30,}\b")),
    ("google_api_key", re.compile(r"\bAIza[0-9A-Za-z_-]{20,}\b")),
    ("aws_access_key", re.compile(r"\bAKIA[0-9A-Z]{16}\b")),
    ("private_key", re.compile(r"-----BEGIN (?:RSA |OPENSSH |EC |)PRIVATE KEY-----")),
]

ALLOW_TOKENS = (
    "...",
    "example",
    "placeholder",
    "change-this",
    "your_",
    "your-",
    "xxxxxxxx",
    "abc",
    "test",
    "dummy",
    "mock",
)


def is_text_file(path: Path) -> bool:
    if path.name in {"Dockerfile", "Dockerfile.goreleaser", "Makefile"}:
        return True
    return path.suffix in TEXT_SUFFIXES or ".env" in path.name


def should_skip(path: Path) -> bool:
    rel_parts = path.relative_to(ROOT).parts
    if any(part in SKIP_DIRS for part in rel_parts):
        return True
    if path.name.endswith(("_test.go", ".spec.ts", ".spec.tsx")):
        return True
    if not is_text_file(path):
        return True
    return False


def is_allowed_line(line: str) -> bool:
    lower = line.lower()
    return any(token in lower for token in ALLOW_TOKENS)


def main() -> int:
    findings: list[str] = []

    for path in ROOT.rglob("*"):
        if not path.is_file() or should_skip(path):
            continue
        try:
            text = path.read_text(encoding="utf-8")
        except UnicodeDecodeError:
            continue

        for line_no, line in enumerate(text.splitlines(), 1):
            if is_allowed_line(line):
                continue
            for name, pattern in SECRET_PATTERNS:
                if pattern.search(line):
                    rel = path.relative_to(ROOT).as_posix()
                    findings.append(f"{rel}:{line_no}: possible {name}")

    if findings:
        print("Secret scan failed:")
        for finding in findings:
            print(f"  {finding}")
        return 1

    print("Secret scan passed.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
