#!/usr/bin/env python3
"""Synthetic staging analytics traffic and collection-coverage checks.

`send` reads each site's published analytics catalog and generates page views:
real headless-Chromium visits (exercising the published beacon script) and
direct bodyless beacon POSTs identical to the script's request (for volume).
Every attempt is appended to a JSONL ledger.

`check` reconciles three levels:
  ledger (what we sent) <= Caddy beacon logs (what was accepted)
  Caddy beacon logs == pond /analytics/<site>/daily (what was counted)
and that /analytics/<site>/public totals match daily totals in its window.
Synthetic unknown pages are visited by nothing else, so for them the ledger
must match the logs exactly.

Staging only: synthetic views are indistinguishable from real ones.
"""

import argparse
import collections
import csv
import http.client
import io
import json
import random
import re
import shlex
import subprocess
import sys
import tempfile
import time
from datetime import datetime, timedelta, timezone
from pathlib import Path
from urllib.parse import urlencode, urlsplit

SITES = {"caspar": "/", "noyo": "/noyo-harbor/"}
UNKNOWN_PREFIX = "/synthetic-unknown/"
OTHER = "Other / unclassified"
TAG = re.compile(r"<script[^>]*data-watertown-analytics[^>]*>")
ATTRS = re.compile(r'data-(endpoint|catalog-id|page-uri)="([^"]*)"')
CHROMIUM_FLAGS = [
    "--headless=new", "--no-sandbox", "--disable-gpu", "--no-first-run",
    "--disable-background-networking", "--disable-component-update", "--disable-sync",
]
DEFAULT_POND = "~/watertown/config/scripts/pond.sh analytics-staging"


def utc_now():
    return datetime.now(timezone.utc)


def rfc3339(moment):
    return moment.isoformat().replace("+00:00", "Z")


def parse_ts(value):
    """Parse Caddy's rfc3339_nano UTC time (fractions beyond microseconds dropped)."""
    if not isinstance(value, str):
        return None
    match = re.fullmatch(r"(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d+))?Z", value)
    if not match:
        return None
    micros = int((match.group(2) or "0")[:6].ljust(6, "0"))
    return datetime.strptime(match.group(1), "%Y-%m-%dT%H:%M:%S").replace(
        microsecond=micros, tzinfo=timezone.utc)


class Http:
    def __init__(self, base_url):
        parts = urlsplit(base_url)
        self.host, self.port = parts.hostname, parts.port
        self.connection_class = (http.client.HTTPSConnection if parts.scheme == "https"
                                 else http.client.HTTPConnection)
        self.connection = None

    def request(self, method, path):
        for attempt in range(2):
            if self.connection is None:
                self.connection = self.connection_class(self.host, self.port, timeout=30)
            try:
                self.connection.request(method, path, headers={"Content-Length": "0"}
                                        if method == "POST" else {})
                response = self.connection.getresponse()
                return response.status, response.read()
            except (http.client.HTTPException, OSError):
                self.connection.close()
                self.connection = None
                if attempt:
                    raise
        raise AssertionError("unreachable")

    def get(self, path):
        status, body = self.request("GET", path)
        if status != 200:
            raise RuntimeError(f"GET {path} returned {status}")
        return body


def load_site(client, site):
    mount = SITES[site]
    catalog = json.loads(client.get(mount + "_analytics/catalog.json"))
    tag = TAG.search(client.get(mount).decode())
    if tag is None:
        raise RuntimeError(f"{site}: {mount} has no analytics script tag")
    attrs = dict(ATTRS.findall(tag.group(0)))
    if attrs.get("catalog-id") != catalog["catalog_id"]:
        raise RuntimeError(f"{site}: page catalog_id does not match catalog.json")
    pages = [entry["page_uri"] for entry in catalog["entries"]]
    if not pages:
        raise RuntimeError(f"{site}: catalog has no pages")
    return {"endpoint": attrs["endpoint"], "catalog_id": catalog["catalog_id"], "pages": pages}


def beacon_path(target, page_uri, catalog_id=None):
    query = urlencode({"page_uri": page_uri, "catalog_id": catalog_id or target["catalog_id"]})
    return f"{target['endpoint']}?{query}"


def browser_visit(chromium, url, budget_ms):
    with tempfile.TemporaryDirectory(prefix="synthetic-visit-") as profile:
        result = subprocess.run(
            [chromium, *CHROMIUM_FLAGS, f"--user-data-dir={profile}",
             f"--virtual-time-budget={budget_ms}", "--dump-dom", url],
            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, timeout=180, check=False,
        )
    return result.returncode == 0 and b"data-watertown-analytics" in result.stdout


def send(args):
    client = Http(args.base_url)
    targets = {site: load_site(client, site) for site in SITES}
    rng = random.Random(args.seed)
    ledger = Path(args.ledger).expanduser()
    ledger.parent.mkdir(parents=True, exist_ok=True)
    counts = collections.Counter()

    def pick_site():
        return "noyo" if rng.random() < args.noyo_share else "caspar"

    with ledger.open("a") as out:
        def record(**entry):
            out.write(json.dumps({"sent_at": rfc3339(utc_now()), **entry}) + "\n")
            out.flush()
            counts[(entry["mode"], entry["status"])] += 1

        for _ in range(args.browser_visits):
            site = pick_site()
            page = rng.choice(targets[site]["pages"])
            sent_at = rfc3339(utc_now())
            ok = browser_visit(args.chromium, args.base_url.rstrip("/") + page, args.budget_ms)
            record(sent_at=sent_at, site=site, mode="browser", page_uri=page,
                   status=204 if ok else None)

        interval = 1.0 / args.rate if args.rate > 0 else 0.0
        next_at = time.monotonic()
        for _ in range(args.beacons):
            site = pick_site()
            if rng.random() < args.unknown_share:
                page = f"{UNKNOWN_PREFIX}{rng.randint(1, args.unknown_pages)}"
            else:
                page = rng.choice(targets[site]["pages"])
            status, _ = client.request("POST", beacon_path(targets[site], page))
            record(site=site, mode="beacon", page_uri=page, status=status)
            next_at += interval
            delay = next_at - time.monotonic()
            if delay > 0:
                time.sleep(delay)

        for index in range(args.invalid):
            site = pick_site()
            target = targets[site]
            path = [
                beacon_path(target, "no-leading-slash"),
                beacon_path(target, "/" + "x" * 1024),
                beacon_path(target, "/", catalog_id="not-a-catalog"),
                target["endpoint"],
            ][index % 4]
            status, _ = client.request("POST", path)
            record(site=site, mode="invalid", page_uri=None, status=status)

    for (mode, status), count in sorted(counts.items(), key=str):
        print(f"{mode:8} status={status}: {count}")
    failed = sum(count for (mode, status), count in counts.items()
                 if status != (400 if mode == "invalid" else 204))
    if failed:
        print(f"FAIL: {failed} requests did not get the expected response", file=sys.stderr)
    return 1 if failed else 0


def log_views(log_root, site):
    """Yield (time, page_uri) for each accepted view in a site's Caddy logs."""
    unreadable = 0
    for path in sorted(Path(log_root, site).glob("beacons*.log")):
        with path.open() as lines:
            for line in lines:
                row = json.loads(line)
                if (row.get("status") != 204 or row.get("site_id") != f"{site}-staging"
                        or row.get("event") != "pageview" or str(row.get("schema_version")) != "1"):
                    continue
                moment = parse_ts(row.get("ts"))
                if moment is None:
                    unreadable += 1
                    continue
                yield moment, row["page_uri"]
    if unreadable:
        raise RuntimeError(f"{site}: {unreadable} accepted views have a non-RFC3339 ts")


def pond_rows(pond, site, node, sql, url=None):
    url = url or f"series:///analytics/{site}/{node}"
    result = subprocess.run(
        [*pond, "cat", url, "--format=csv", "--sql", sql],
        capture_output=True, text=True, check=False,
    )
    if result.returncode != 0:
        raise RuntimeError(f"pond cat {url} failed:\n{result.stderr[-2000:]}")
    return list(csv.DictReader(io.StringIO(result.stdout)))


def check_site(args, pond, site, ledger_entries):
    problems = []
    views = list(log_views(args.log_root, site))
    cutoff_rows = pond_rows(pond, site, "views",
                            "SELECT CAST(max(timestamp) AS VARCHAR) AS cutoff FROM source")
    cutoff_text = cutoff_rows[0]["cutoff"] if cutoff_rows else ""
    if not cutoff_text:
        return [f"{site}: pond has no views yet"], {}
    # The newest ingested view; later log lines await the next ingest.
    cutoff = datetime.fromisoformat(cutoff_text[:26]).replace(tzinfo=timezone.utc)

    logged = collections.Counter((moment.date().isoformat(), page)
                                 for moment, page in views if moment <= cutoff)
    daily = collections.Counter()
    for row in pond_rows(pond, site, "daily",
                         "SELECT CAST(CAST(timestamp AS DATE) AS VARCHAR) AS day, page_uri, "
                         "CAST(views AS BIGINT) AS views FROM source"):
        daily[(row["day"], row["page_uri"])] += int(row["views"])
    for key in sorted(set(logged) | set(daily)):
        if logged[key] != daily[key]:
            problems.append(f"{site}: {key[0]} {key[1]}: Caddy log {logged[key]} != pond {daily[key]}")

    # Classify logged views with the same inventory /analytics/<site>/public joins.
    sections = collections.defaultdict(set)
    for row in pond_rows(pond, site, None, "SELECT page_uri, section FROM source",
                         url=f"file+table:///analytics/{site}/pages.parquet"):
        sections[row["page_uri"]].add(row["section"])
    today = utc_now().date().isoformat()
    window_start = (utc_now().date() - timedelta(days=29)).isoformat()
    expected = collections.Counter()
    for (day, page), count in logged.items():
        if window_start <= day <= today:
            section = sections.get(page, set())
            expected[(day, next(iter(section)) if len(section) == 1 else OTHER)] += count
    public = collections.Counter()
    for row in pond_rows(pond, site, "public",
                         "SELECT CAST(CAST(timestamp AS DATE) AS VARCHAR) AS day, section, "
                         "CAST(views AS BIGINT) AS views FROM source"):
        public[(row["day"], row["section"])] += int(row["views"])
    for key in sorted(set(expected) | set(public)):
        if expected[key] != public[key]:
            problems.append(f"{site}: public {key[0]} {key[1]}: "
                            f"Caddy log {expected[key]} != pond {public[key]}")

    accepted = [entry for entry in ledger_entries
                if entry["site"] == site and entry["mode"] in ("browser", "beacon")
                and entry["status"] == 204]
    if accepted:
        first = min(parse_ts(entry["sent_at"]) for entry in accepted) - timedelta(seconds=5)
        last = max(parse_ts(entry["sent_at"]) for entry in accepted) + timedelta(minutes=5)
        sent = collections.Counter(entry["page_uri"] for entry in accepted)
        seen = collections.Counter(page for moment, page in views if first <= moment <= last)
        for page in sorted(sent):
            exact = page.startswith(UNKNOWN_PREFIX)
            if seen[page] < sent[page] or (exact and seen[page] != sent[page]):
                problems.append(f"{site}: {page}: ledger {sent[page]} vs Caddy log {seen[page]}")

    summary = {
        "logged": len(views), "counted_until": cutoff_text,
        "pond_views": sum(daily.values()), "public_30d": sum(public.values()),
        "unclassified_30d": sum(count for (_, section), count in public.items()
                                if section == OTHER),
        "ledger_accepted": len(accepted),
        "awaiting_ingest": sum(1 for moment, _ in views if moment > cutoff),
    }
    return problems, summary


def load_ledger(path):
    path = Path(path).expanduser()
    if not path.exists():
        return []
    with path.open() as lines:
        return [json.loads(line) for line in lines if line.strip()]


def check(args):
    pond = [str(Path(part).expanduser()) if part.startswith("~") else part
            for part in shlex.split(args.pond)]
    ledger_entries = load_ledger(args.ledger)
    problems = []
    for site in SITES:
        site_problems, summary = check_site(args, pond, site, ledger_entries)
        problems += site_problems
        print(f"{site}: " + ", ".join(f"{key}={value}" for key, value in summary.items()))
    for problem in problems:
        print(f"FAIL: {problem}", file=sys.stderr)
    if not problems:
        print("OK: ledger <= Caddy logs == pond daily; public sections match the catalog")
    return 1 if problems else 0


def main():
    parser = argparse.ArgumentParser(description=__doc__,
                                     formatter_class=argparse.RawDescriptionHelpFormatter)
    parser.add_argument("--ledger", default="~/analytics-synthetic/ledger.jsonl")
    commands = parser.add_subparsers(dest="command", required=True)

    send_parser = commands.add_parser("send", help="generate synthetic views")
    send_parser.add_argument("--base-url", default="http://localhost")
    send_parser.add_argument("--browser-visits", type=int, default=0)
    send_parser.add_argument("--beacons", type=int, default=0)
    send_parser.add_argument("--invalid", type=int, default=0,
                             help="malformed beacons that Caddy must reject")
    send_parser.add_argument("--rate", type=float, default=20.0, help="direct beacons per second")
    send_parser.add_argument("--noyo-share", type=float, default=0.25)
    send_parser.add_argument("--unknown-share", type=float, default=0.05,
                             help=f"share of direct beacons for uncataloged {UNKNOWN_PREFIX}N pages")
    send_parser.add_argument("--unknown-pages", type=int, default=3)
    send_parser.add_argument("--chromium", default="chromium")
    send_parser.add_argument("--budget-ms", type=int, default=3000)
    send_parser.add_argument("--seed", type=int)

    check_parser = commands.add_parser("check", help="reconcile ledger, Caddy logs, and pond")
    check_parser.add_argument("--log-root", default="/var/log/caddy/beacons/staging")
    check_parser.add_argument("--pond", default=DEFAULT_POND,
                              help="command prefix that runs pond against analytics-staging")

    args = parser.parse_args()
    return send(args) if args.command == "send" else check(args)


if __name__ == "__main__":
    sys.exit(main())
