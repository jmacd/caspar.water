#!/usr/bin/env python3
"""Flatten the hand-maintained monthly.csv into a tidy time series.

The source file (history/monthly.csv) is a spreadsheet export with one block
per year separated by blank rows. Each block starts with a header row whose
column B holds the 4-digit year; the twelve following month rows carry the
cumulative meter readings we care about:

    column A  month name              (January .. December)
    column B  main service meter      (cumulative gallons)
    column E  community center meter  (cumulative gallons)

Total/average rows and the decorative "Community"/blank rows are ignored.
Cell values may contain spaces and thousands separators; parentheses, a bare
dash, "ND", or an empty cell all mean "no reading".

Output (history/monthly_usage.csv) is a tidy CSV, one row per month with an
RFC3339 timestamp so the DataFusion CSV provider infers a real timestamp
column:

    timestamp,main_meter,community_center
    2017-01-01T00:00:00Z,50586850,1593373
    ...

Usage: python3 history/monthly_to_usage.py [input.csv] [output.csv]
"""
from __future__ import annotations

import csv
import datetime
import re
import sys

MONTHS = {
    m: i + 1
    for i, m in enumerate(
        [
            "January",
            "February",
            "March",
            "April",
            "May",
            "June",
            "July",
            "August",
            "September",
            "October",
            "November",
            "December",
        ]
    )
}

_YEAR_RE = re.compile(r"(?:19|20)\d\d")


def parse_number(cell: str | None) -> int | None:
    """Parse a messy numeric cell into an int, or None when there is no value."""
    if cell is None:
        return None
    s = cell.strip()
    if s in ("", "-", "ND"):
        return None
    # Strip parentheses (spreadsheet negatives) and thousands separators.
    s = re.sub(r"[(),]", "", s).strip()
    if s in ("", "-"):
        return None
    try:
        return int(round(float(s)))
    except ValueError:
        return None


def flatten(rows: list[list[str]]) -> list[tuple[str, int | None, int | None]]:
    out: list[tuple[str, int | None, int | None]] = []
    year: int | None = None
    for row in rows:
        if not any(cell.strip() for cell in row):
            continue  # blank separator row
        col_a = row[0].strip()
        col_b = row[1].strip() if len(row) > 1 else ""
        # A block header row carries the year in column B.
        if _YEAR_RE.fullmatch(col_b):
            year = int(col_b)
            continue
        if year is None or col_a not in MONTHS:
            continue  # Total/average/decoration rows
        ts = datetime.datetime(
            year, MONTHS[col_a], 1, tzinfo=datetime.timezone.utc
        ).strftime("%Y-%m-%dT%H:%M:%SZ")
        main = parse_number(row[1]) if len(row) > 1 else None
        community = parse_number(row[4]) if len(row) > 4 else None
        out.append((ts, main, community))
    return out


def main(argv: list[str]) -> int:
    src = argv[1] if len(argv) > 1 else "history/monthly.csv"
    dst = argv[2] if len(argv) > 2 else "history/monthly_usage.csv"

    with open(src, newline="") as f:
        rows = list(csv.reader(f))

    records = flatten(rows)

    with open(dst, "w", newline="") as f:
        w = csv.writer(f)
        w.writerow(["timestamp", "main_meter", "community_center"])
        for ts, main, community in records:
            w.writerow(
                [
                    ts,
                    "" if main is None else main,
                    "" if community is None else community,
                ]
            )

    print(f"wrote {len(records)} rows to {dst}")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
