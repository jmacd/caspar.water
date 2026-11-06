#!/usr/bin/env python3
"""Exercise deployment scripts without containers, services, or deployed data."""

import fcntl
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest


class AnalyticsReportTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.scripts = self.root / "config/scripts"
        self.scripts.mkdir(parents=True)
        for name in ("pond.sh", "run-analytics-report.sh"):
            shutil.copyfile(Path(__file__).parent / name, self.scripts / name)
            (self.scripts / name).chmod(0o700)
        for name in ("env", "bin", "runtime", "reports", "catalogs"):
            (self.root / name).mkdir()
        for instance in ("site-staging", "noyo-staging"):
            (self.root / "reports" / instance).mkdir()
        self.log = self.root / "calls.jsonl"
        self.env = {
            "PATH": f"{self.root / 'bin'}:{os.defpath}",
            "HOME": str(self.root),
            "XDG_RUNTIME_DIR": str(self.root / "runtime"),
            "CALL_LOG": str(self.log),
        }
        stub = f"""#!{sys.executable}
import fcntl, json, os, sys
from pathlib import Path
name = Path(sys.argv[0]).name
with open(os.environ["CALL_LOG"], "a") as log:
    log.write(json.dumps([name] + sys.argv[1:]) + "\\n")
if name == "flock":
    try:
        fcntl.flock(9, fcntl.LOCK_EX | fcntl.LOCK_NB)
    except BlockingIOError:
        sys.exit(75)
else:
    key = "PULL_EXIT" if sys.argv[1] == "pull" else "RUN_EXIT"
    sys.exit(int(os.environ.get(key, "0")))
"""
        for name in ("podman", "flock"):
            path = self.root / "bin" / name
            path.write_text(stub)
            path.chmod(0o700)

    def configure(self, instance="noyo-staging", **overrides):
        values = {
            "POND_VOLUME": f"pond-{instance}",
            "ANALYTICS_REPORT_ENABLED": "1",
            "ANALYTICS_REPORT_ROOT": str(self.root / "reports"),
            "ANALYTICS_CATALOG_DIR": str(self.root / "catalogs"),
        }
        values.update(overrides)
        (self.root / "env" / f"{instance}.env").write_text(
            "".join(f"{key}={shlex.quote(value)}\n" for key, value in values.items())
        )

    def invoke(self, script, *args):
        return subprocess.run(
            ["bash", str(self.scripts / script), *args],
            env=self.env, capture_output=True, text=True, check=False,
        )

    def calls(self):
        return [json.loads(line) for line in self.log.read_text().splitlines()] if self.log.exists() else []

    def test_unconfigured_wrapper_is_unchanged(self):
        for instance in ("noyo-staging", "noyo-prod"):
            with self.subTest(instance=instance):
                self.configure(instance, ANALYTICS_REPORT_ENABLED="0", ANALYTICS_REPORT_ROOT="")
                result = self.invoke("pond.sh", instance, "run", "/etc/ingest")
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertFalse(any("/analytics-" in arg for arg in self.calls()[-1]))

    def test_renderer_mounts_saved_reports_read_only_without_catalogs(self):
        self.configure("site-staging")
        result = self.invoke("pond.sh", "site-staging", "run", "/system/etc/90-sitegen", "build", "/www")
        self.assertEqual(result.returncode, 0, result.stderr)
        args = self.calls()[0]
        self.assertIn(f"{self.root / 'reports'}:/analytics-reports:ro", args)
        self.assertFalse(any("/analytics-catalogs" in arg for arg in args))

    def test_refresh_locks_pulls_and_writes_only_its_own_report(self):
        self.configure()
        result = self.invoke("run-analytics-report.sh", "noyo-staging")
        self.assertEqual(result.returncode, 0, result.stderr)
        calls = self.calls()
        self.assertEqual(calls[0], ["flock", "-w", "600", "9"])
        self.assertEqual(calls[1][:2], ["podman", "pull"])
        args = calls[2]
        self.assertIn(f"{self.root / 'reports/noyo-staging'}:/analytics-reports/noyo-staging", args)
        self.assertIn(f"{self.root / 'catalogs'}:/analytics-catalogs:ro", args)
        self.assertNotIn(f"{self.root / 'reports'}:/analytics-reports:ro", args)
        self.assertFalse(any("site-staging" in arg for arg in args))
        self.assertEqual(args[-3:], ["run", "/system/etc/90-sitegen", "analytics-report"])
        self.assertTrue((self.root / "runtime" / f"watertown-{os.getuid()}-noyo-staging.lock").exists())

    def test_disabled_and_native_jobs_fail_before_pulling(self):
        for overrides in ({"ANALYTICS_REPORT_ENABLED": "0"}, {"POND_RUNTIME": "native"}):
            with self.subTest(overrides=overrides):
                self.configure(**overrides)
                for script, extra in (
                    ("run-analytics-report.sh", []),
                    ("pond.sh", ["--analytics-report"]),
                ):
                    result = self.invoke(script, "noyo-staging", *extra)
                    self.assertNotEqual(result.returncode, 0)
                    self.assertIn("ERROR:", result.stderr)
                    self.assertEqual(self.calls(), [])

    def test_production_is_rejected_by_both_entrypoints(self):
        self.configure("noyo-prod")
        for script, args in (
            ("run-analytics-report.sh", ["noyo-prod"]),
            ("pond.sh", ["noyo-prod", "--analytics-report"]),
            ("pond.sh", ["noyo-prod", "run", "/system/etc/90-sitegen", "build", "/www"]),
        ):
            with self.subTest(script=script, args=args):
                result = self.invoke(script, *args)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("staging", result.stderr)
                self.assertEqual(self.calls(), [])

    def test_missing_private_directories_and_extra_arguments_fail(self):
        for overrides, extra in (
            ({"ANALYTICS_REPORT_ROOT": ""}, []),
            ({"ANALYTICS_REPORT_ROOT": "relative"}, []),
            ({"ANALYTICS_REPORT_ROOT": str(self.root / "missing")}, []),
            ({"ANALYTICS_REPORT_ROOT": str(self.root / "catalogs")}, []),
            ({"ANALYTICS_CATALOG_DIR": str(self.root / "missing")}, []),
            ({}, ["run", "/etc/other"]),
        ):
            with self.subTest(overrides=overrides, extra=extra):
                self.configure(**overrides)
                result = self.invoke("pond.sh", "noyo-staging", "--analytics-report", *extra)
                self.assertNotEqual(result.returncode, 0)
                self.assertIn("ERROR:", result.stderr)
                self.assertEqual(self.calls(), [])

    def test_refresh_failure_surfaces_without_modifying_saved_state(self):
        self.configure()
        state = self.root / "reports/noyo-staging/report-state.json"
        state.write_text('{"fixture": "last-good"}')
        self.env["RUN_EXIT"] = "42"
        result = self.invoke("run-analytics-report.sh", "noyo-staging")
        self.assertEqual(result.returncode, 42)
        self.assertEqual(state.read_text(), '{"fixture": "last-good"}')

    def test_pull_failure_does_not_attempt_refresh(self):
        self.configure()
        self.env["PULL_EXIT"] = "43"
        result = self.invoke("run-analytics-report.sh", "noyo-staging")
        self.assertEqual(result.returncode, 43)
        self.assertEqual(len(self.calls()), 2)

    def test_existing_pond_lock_prevents_any_container_command(self):
        self.configure()
        lock = self.root / "runtime" / f"watertown-{os.getuid()}-noyo-staging.lock"
        with lock.open("w") as held:
            fcntl.flock(held, fcntl.LOCK_EX | fcntl.LOCK_NB)
            result = self.invoke("run-analytics-report.sh", "noyo-staging")
        self.assertEqual(result.returncode, 75)
        self.assertEqual(len(self.calls()), 1)


if __name__ == "__main__":
    unittest.main()
