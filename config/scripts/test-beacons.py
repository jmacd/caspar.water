#!/usr/bin/env python3
"""Exercise separate Caddy logs and the ordinary pond ingestion/query path."""

import http.client
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tempfile
import time
import unittest
from urllib.parse import urlencode


ROOT = Path(__file__).resolve().parents[2]
CATALOG = "a" * 64


class Beacons(unittest.TestCase):
    def setUp(self):
        self.caddy = shutil.which(os.environ.get("CADDY_BIN", "caddy"))
        self.assertIsNotNone(self.caddy, "Set CADDY_BIN to a Caddy 2.10+ executable")
        self.temp = tempfile.TemporaryDirectory(prefix="caddy-beacons-")
        self.addCleanup(self.temp.cleanup)
        self.work = Path(self.temp.name)
        self.output = (self.work / "caddy.stderr").open("w+")
        self.addCleanup(self.output.close)
        with socket.socket() as sock:
            sock.bind(("127.0.0.1", 0))
            self.port = sock.getsockname()[1]
        (self.work / "www").mkdir()
        (self.work / "www" / "water.html").write_text("ordinary page")
        self.config = self.work / "Caddyfile"
        self.config.write_text(f"""
{{
    admin off
    auto_https off
}}
import "{ROOT / 'config/caddy/beacons.caddy'}"
http://127.0.0.1:{self.port} {{
    log ordinary {{
        output file "{self.work}/access.log"
        format json
    }}
    import analytics-beacons site-a /_analytics/a/view "{self.work}/a"
    import analytics-beacons site-b /_analytics/b/view "{self.work}/b"
    handle {{
        root * "{self.work}/www"
        try_files {{path}} {{path}}.html {{path}}/
        file_server
    }}
}}
""")
        self.process = None
        self.addCleanup(self.stop)
        self.start()

    def start(self):
        self.process = subprocess.Popen(
            [self.caddy, "run", "--config", str(self.config), "--adapter", "caddyfile"],
            stdout=self.output, stderr=self.output,
        )
        for _ in range(100):
            if self.process.poll() is not None:
                self.fail((self.work / "caddy.stderr").read_text())
            try:
                if self.request("GET", "/water")[0] == 200:
                    return
            except OSError:
                pass
            time.sleep(0.05)
        self.fail("Caddy did not become ready")

    def stop(self):
        if self.process is not None and self.process.poll() is None:
            self.process.terminate()
            try:
                self.process.wait(timeout=10)
            except subprocess.TimeoutExpired:
                self.process.kill()
                self.process.wait(timeout=5)

    def request(self, method, path, body=None, headers=None):
        connection = http.client.HTTPConnection("127.0.0.1", self.port, timeout=5)
        try:
            connection.request(method, path, body=body, headers=headers or {})
            response = connection.getresponse()
            return response.status, response.read(), dict(response.getheaders())
        finally:
            connection.close()

    def beacon(self, site="a", page="/water", **extra):
        query = urlencode({"page_uri": page, "catalog_id": CATALOG, **extra})
        return self.request("POST", f"/_analytics/{site}/view?{query}", headers={
            "Cookie": "visitor=private-cookie",
            "Referer": "https://private-referrer.invalid/secret",
            "User-Agent": "private-user-agent",
            "Authorization": "Bearer private-authorization",
        })

    def records(self, relative, count):
        path = self.work / relative
        for _ in range(100):
            rows = [json.loads(line) for line in path.read_text().splitlines()] if path.exists() else []
            if len(rows) >= count:
                return rows
            time.sleep(0.02)
        self.fail(f"Expected {count} records in {relative}")

    def test_logging_is_private_separate_and_site_scoped(self):
        status, body, headers = self.beacon(
            page="/blog/café", site_id="site-b", visitor_id="private-query",
        )
        self.assertEqual((status, body), (204, b""))
        self.assertEqual(headers["Cache-Control"], "no-store")
        self.assertEqual(self.beacon("b", "/noyo-harbor/")[0], 204)
        self.assertEqual(self.request("GET", "/water")[:2], (200, b"ordinary page"))
        for method, query in [
            ("GET", urlencode({"page_uri": "/water", "catalog_id": CATALOG})),
            ("POST", ""),
            ("POST", urlencode({"page_uri": "not-a-path", "catalog_id": CATALOG})),
            ("POST", urlencode({"page_uri": "/" + "x" * 1024, "catalog_id": CATALOG})),
            ("POST", urlencode({"page_uri": "/water", "catalog_id": "invalid"})),
        ]:
            self.assertEqual(self.request(method, f"/_analytics/a/view?{query}")[0], 400)
        a = self.records("a/beacons.log", 6)
        b = self.records("b/beacons.log", 1)
        self.assertEqual([row["site_id"] for row in a], ["site-a"] * 6)
        self.assertEqual(b[0]["site_id"], "site-b")
        self.assertEqual(a[0]["page_uri"], "/blog/café")
        self.assertEqual(a[0]["catalog_id"], CATALOG)
        self.assertEqual(str(a[0]["schema_version"]), "1")
        self.assertEqual(a[0]["event"], "pageview")
        self.assertIsInstance(a[0]["ts"], (int, float))
        self.assertLess(a[0]["ts"], 10**11)
        self.assertEqual([row["status"] for row in a], [204] + [400] * 5)
        for row in a + b:
            self.assertNotIn("request", row)
            self.assertNotIn("resp_headers", row)
            self.assertNotIn("user_id", row)
        for row in a[1:]:
            self.assertNotIn("page_uri", row)
            self.assertNotIn("catalog_id", row)
        self.assertNotIn("private-", json.dumps(a + b))
        ordinary = self.records("access.log", 2)
        self.assertTrue(all("/_analytics/" not in row["request"]["uri"] for row in ordinary))

    def test_pond_ingestion_append_rotation_and_daily_counts(self):
        pond = Path(os.environ.get("POND_BIN", ROOT / "watertown/target/debug/pond")).resolve()
        self.assertTrue(pond.is_file(), "Build pond or set POND_BIN")
        env = {
            **os.environ,
            "POND": str(self.work / "pond"),
            "ANALYTICS_LOG_DIR": str(self.work / "a"),
            "RUST_LOG": "warn",
        }

        def run(*args, binary=False):
            result = subprocess.run(
                [str(pond), *args], env=env, text=not binary, capture_output=True, timeout=120,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            return result.stdout

        def ingest_and_count(expected):
            run("run", "/etc/analytics-ingest", "push")
            output = run("cat", "/analytics/daily", "--format=table", "--sql",
                         "SELECT CAST(SUM(views) AS BIGINT) AS views FROM source")
            self.assertEqual(re.findall(r"^\|\s*(\d+)\s*\|$", output, re.MULTILINE),
                             [str(expected)], output)

        self.assertEqual(self.beacon()[0], 204)
        self.assertEqual(self.beacon("b")[0], 204)
        self.assertEqual(self.request("POST", "/_analytics/a/view")[0], 400)
        self.records("a/beacons.log", 2)
        run("init", "--birthplace", "beacon-test")
        run("apply", "-f", str(ROOT / "config/analytics.yaml"))
        ingest_and_count(1)
        self.assertTrue(list((self.work / "pond" / "cache").glob(
            "merged_*/res*/manifest.json",
        )), "daily counts must use the shared temporal-reduction cache")
        ingest_and_count(1)
        self.assertEqual(self.beacon(page="/blog/café")[0], 204)
        self.records("a/beacons.log", 3)
        ingest_and_count(2)
        self.stop()
        (self.work / "a/beacons.log").rename(self.work / "a/beacons-2026-11-06T00-00-00.log")
        (self.work / "a/beacons.log").touch()
        self.start()
        self.assertEqual(self.beacon()[0], 204)
        self.records("a/beacons.log", 1)
        ingest_and_count(3)
        ingest_and_count(3)
        output = run("cat", "/analytics/daily", "--format=table")
        self.assertIn("/blog/café", output)
        self.assertIn("/water", output)
        self.stop()
        template = self.records("a/beacons.log", 1)[0]
        with (self.work / "a/beacons.log").open("a") as log:
            for timestamp in [86399.75, 86400.25]:
                log.write(json.dumps({
                    **template, "ts": timestamp, "page_uri": "/day-boundary",
                }) + "\n")
        ingest_and_count(5)
        output = run("cat", "/analytics/daily", "--format=table", "--sql",
                     "SELECT timestamp, views FROM source WHERE page_uri = '/day-boundary'")
        self.assertIn("1970-01-01", output)
        self.assertIn("1970-01-02", output)

        self.start()
        self.assertEqual(self.beacon(page="/unrecognized", section="Noyo")[0], 204)
        self.records("a/beacons.log", 4)
        self.stop()
        today = int(time.time() // 86400) * 86400
        with (self.work / "a/beacons.log").open("a") as log:
            for timestamp in [today - 29 * 86400 + 0.25,
                              today - 29 * 86400 - 0.25, today + 86400 + 0.25]:
                log.write(json.dumps({
                    **template, "ts": timestamp, "page_uri": "/water",
                }) + "\n")
        ingest_and_count(9)
        inventory_source = self.work / "pages-source.jsonl"
        inventory = self.work / "pages.parquet"

        def publish_inventory(rows):
            inventory_source.write_text("".join(json.dumps(row) + "\n" for row in rows))
            run("copy", inventory_source.as_uri().replace("file:", "host:"),
                "/analytics/pages-source.jsonl")
            inventory.write_bytes(run(
                "cat", "jsonlogs:///analytics/pages-source.jsonl", "--format=raw", binary=True,
            ))
            run("copy", inventory.as_uri().replace("file:", "host:"), "/analytics/pages.parquet")

        static = {"site_id": "site-a", "page_uri": "/water", "section": "Static content"}
        blog = {"site_id": "site-a", "page_uri": "/blog/café", "section": "Blog"}
        other_site = {"site_id": "site-b", "page_uri": "/unrecognized", "section": "Noyo"}
        publish_inventory([static, static, blog, other_site])
        run("apply", "-f", str(ROOT / "config/analytics-public.yaml"))

        def section_counts():
            output = run("cat", "/analytics/public/daily-sections", "--format=table", "--sql",
                         "SELECT section, SUM(views) AS views FROM source GROUP BY section ORDER BY section")
            return dict(re.findall(
                r"^\|\s*(Blog|Static content|Monitoring|Other / unclassified|Noyo)\s*\|\s*(\d+)\s*\|$",
                output, re.MULTILINE,
            ))

        self.assertEqual(section_counts(), {
            "Blog": "1", "Other / unclassified": "1", "Static content": "3",
        })
        public = run("cat", "/analytics/public/daily-sections", "--format=table")
        for private in ["page_uri", "site_id", "catalog_id", "/unrecognized", "/water", "/blog/café"]:
            self.assertNotIn(private, public)
        self.assertIn("timestamp", public)
        self.assertIn("section", public)
        self.assertIn("views", public)

        publish_inventory([{**static, "section": "Monitoring"}, other_site])
        self.assertEqual(section_counts(), {
            "Monitoring": "3", "Other / unclassified": "2",
        })

        empty_page = self.work / "empty-page.md"
        empty_page.write_text("---\ntitle: Excluded page\nanalytics: false\n---\nNo analytics.\n")
        run("copy", empty_page.as_uri().replace("file:", "host:"), "/empty-page.md")
        empty_config = self.work / "empty-site.yaml"
        empty_config.write_text(
            "site: {title: Empty inventory}\n"
            "analytics: {enabled: true, site_id: site-a}\n"
            "documentation: {version: 1, source_root: /}\n"
            "routes:\n"
            "  - {name: home, type: static, slug: '', page: /empty-page.md}\n"
        )
        empty_site = self.work / "empty-site"
        run("mknod", "sitegen", "/etc/empty-site", "--config-path", str(empty_config))
        run("run", "/etc/empty-site", "build", str(empty_site))
        empty_inventory = empty_site / "_analytics/pages.parquet"
        run("copy", empty_inventory.as_uri().replace("file:", "host:"),
            "/analytics/pages.parquet")
        self.assertEqual(section_counts(), {"Other / unclassified": "5"})

        publish_inventory([static, blog])
        self.assertEqual(section_counts(), {
            "Blog": "1", "Other / unclassified": "1", "Static content": "3",
        })

        inventory.write_bytes(b"not Parquet")
        run("copy", inventory.as_uri().replace("file:", "host:"), "/analytics/pages.parquet")
        failed = subprocess.run(
            [str(pond), "cat", "/analytics/public/daily-sections", "--format=table"],
            env=env, text=True, capture_output=True, timeout=120,
        )
        self.assertNotEqual(failed.returncode, 0, failed.stdout)
        self.assertIn("Parquet", failed.stderr)

    def test_staging_pond_keeps_sites_separate_and_builds_two_reports(self):
        pond = Path(os.environ.get("POND_BIN", ROOT / "watertown/target/debug/pond")).resolve()
        env = {**os.environ, "POND": str(self.work / "analytics"), "RUST_LOG": "warn"}

        def run(*args):
            result = subprocess.run(
                [str(pond), *args], env=env, text=True, capture_output=True, timeout=120,
            )
            self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
            return result.stdout

        run("init", "--birthplace", "analytics-staging-test")
        template = (ROOT / "config/analytics/site-pond.yaml.tftpl").read_text()
        now = time.time()
        for site, count, base in [("caspar", 2, "/"), ("noyo", 3, "/noyo-harbor/")]:
            logs = self.work / "logs" / site
            logs.mkdir(parents=True)
            event = {"ts": now, "schema_version": 1, "event": "pageview", "status": 204,
                     "site_id": f"{site}-staging", "page_uri": base}
            (logs / "beacons.log").write_text(
                "".join(json.dumps(event) + "\n" for _ in range(count))
                + json.dumps({**event, "site_id": "wrong-site"}) + "\n"
            )
            config = self.work / f"{site}-pond.yaml"
            config.write_text(template.replace("${site}", site).replace("${title}", site)
                              .replace("/beacons/", str(self.work / "logs") + "/"))
            run("apply", "-f", str(config))
            run("run", f"/etc/ingest-{site}", "push")
            source = self.work / f"{site}-source"
            source.mkdir()
            (source / "index.md").write_text("---\ntitle: Page\n---\nContent.\n")
            (source / "site.yaml").write_text(
                f"site: {{title: {site}, base_url: {base}}}\n"
                "analytics:\n  enabled: true\n"
                f"  site_id: {site}-staging\n  default_section: {site}\n"
                f"  endpoint: /_analytics/{site}/view\n"
                "documentation: {version: 1, source_root: /}\n"
                "routes:\n  - {name: home, type: static, slug: '', page: /index.md}\n"
            )
            published = self.work / f"{site}-published"
            run("run", "-d", str(source), "host+sitegen:///site.yaml", "--",
                "build", str(published))
            inventory = published / "_analytics/pages.parquet"
            run("copy", inventory.as_uri().replace("file:", "host:"),
                f"/analytics/{site}/pages.parquet")
            public = run("cat", f"/analytics/{site}/public", "--format=csv")
            self.assertIn(f",{site},{count}", public)
            for private in ["page_uri", "site_id", "wrong-site"]:
                self.assertNotIn(private, public)

        parent = self.work / "caspar-source"
        shutil.copytree(self.work / "noyo-source", parent / "noyo")
        with (parent / "site.yaml").open("a") as config:
            config.write(
                "subsites:\n  - {name: noyo, path: /noyo, config: /site.yaml, "
                "base_url: /noyo-harbor/}\n"
            )
        run("run", "-d", str(parent), "host+sitegen:///site.yaml", "--",
            "build", str(self.work / "caspar-published"))
        for mount, site in [("", "caspar"), ("noyo-harbor", "noyo")]:
            catalog = self.work / "caspar-published" / mount / "_analytics/catalog.json"
            self.assertEqual(json.loads(catalog.read_text())["site_id"], f"{site}-staging")

        run("mkdir", "-p", "/analytics/templates")
        run("copy", (ROOT / "config/analytics/site/overview.md").as_uri().replace("file:", "host:"),
            "/analytics/templates/overview.md")
        for site in ["caspar", "noyo"]:
            report = self.work / f"{site}-report"
            run("run", f"/etc/site-{site}", "--", "build", str(report))
            html = (report / "index.html").read_text()
            self.assertIn(f"{site} activity", html)
            self.assertIn('class="top-bar-back" href="/"', html)
            self.assertIn('href="../caspar/"', html)
            self.assertIn('href="../noyo/"', html)
            self.assertIn('class="stacked-area-data"', html)
            self.assertNotIn("data-watertown-analytics", html)
            self.assertTrue(list((report / "data").rglob("*.parquet")))
            unavailable = self.work / f"{site}-unavailable"
            run("run", f"/etc/site-{site}", "--", "build", str(unavailable), "--quick")
            unavailable_html = (unavailable / "index.html").read_text()
            self.assertIn("No reported data available.", unavailable_html)
            self.assertNotIn('class="stacked-area-data"', unavailable_html)
            self.assertFalse(list(unavailable.rglob("*.parquet")))

        source = self.work / "caspar-source"
        (source / "index.md").write_text("---\ntitle: Page\nanalytics: false\n---\nContent.\n")
        published = self.work / "caspar-published"
        run("run", "-d", str(source), "host+sitegen:///site.yaml", "--", "build", str(published))
        run("copy", (published / "_analytics/pages.parquet").as_uri().replace("file:", "host:"),
            "/analytics/caspar/pages.parquet")
        self.assertIn(",Other / unclassified,2",
                      run("cat", "/analytics/caspar/public", "--format=csv"))
        self.assertIn(",noyo,3", run("cat", "/analytics/noyo/public", "--format=csv"))


if __name__ == "__main__":
    unittest.main()
