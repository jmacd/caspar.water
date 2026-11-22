#!/usr/bin/env python3
"""Check selfmon wiring with `python3 config/scripts/test-selfmon.py` (PyYAML)."""

from pathlib import Path
import re
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]


class SelfmonConfiguration(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        terraform = (ROOT / "terraform/station/watershop/watershop.tf").read_text()
        # Read the literal, terraform-fmt-formatted instance map, not credentials.
        block = re.search(r"(?ms)^  instances = \{\n(.*?)^  \}", terraform)
        if block is None:
            raise AssertionError("Cannot locate Terraform's literal local.instances map")
        cls.instances = set(re.findall(r"(?m)^    ([\w-]+)\s*=\s*\{", block[1]))
        if not cls.instances:
            raise AssertionError("Terraform instance inventory is empty")
        documents = list(yaml.safe_load_all(
            (ROOT / "config/watershop-selfmon.yaml").read_text()
        ))
        nodes = [doc for doc in documents if doc.get("kind") == "mknod"]
        cls.nodes = {doc["metadata"]["path"]: doc["spec"] for doc in nodes}
        if len(cls.nodes) != len(nodes):
            raise AssertionError("Duplicate mknod paths")
        entries = cls.nodes["/derived"]["config"]["entries"]
        cls.derived = {entry["name"]: entry for entry in entries}
        if len(cls.derived) != len(entries):
            raise AssertionError("Duplicate derived node names")
        cls.inputs = cls.derived["perf"]["config"]["inputs"]
        cls.sitegen = cls.nodes["/system/etc/sitegen"]["config"]

    def test_every_configured_instance_has_a_complete_perf_path(self):
        scopes = {item["scope"]: item["pattern"] for item in self.inputs}
        self.assertEqual(len(scopes), len(self.inputs), "Duplicate perf join scopes")
        for name in sorted(self.instances | set(scopes) | {"_self"}):
            with self.subTest(instance=name):
                ingest_path = f"/system/etc/measure/{name}"
                self.assertIn(ingest_path, set(self.nodes), "Discovered pond has no ingest node")
                ingest = self.nodes[ingest_path]
                self.assertEqual(ingest["factory"], "logfile-ingest")
                config = ingest["config"]
                self.assertEqual(config["pond_path"], "/measure")
                self.assertEqual(config["active_pattern"],
                                 f"${{env:SELFMON_METRICS_DIR}}/{name}.jsonl")
                self.assertEqual(config["archived_pattern"],
                                 f"${{env:SELFMON_METRICS_DIR}}/{name}.jsonl.*")
                self.assertEqual((config["timestamp_field"], config["timestamp_unit"]),
                                 ("ts", "rfc3339"))

                projection_name = f"p-{name}"
                self.assertIn(projection_name, set(self.derived), "Missing typed perf projection")
                projection = self.derived[projection_name]
                self.assertEqual(projection["factory"], "sql-derived-series")
                self.assertEqual(projection["config"]["patterns"],
                                 {"m": f"jsonlogs:///measure/{name}.jsonl"})
                self.assertEqual(projection["config"]["locality"],
                                 {"type": "timestamp-local", "event_time": "timestamp",
                                  "input_event_time": "ts"})
                if name != "_self":
                    self.assertEqual(
                        projection["config"]["query"],
                        self.derived["p-noyo-staging"]["config"]["query"],
                        "Regular ponds must use the shared measurement schema",
                    )
                expected = f"series:///derived/{projection_name}"
                self.assertEqual(scopes.get(name), expected, "Missing or miswired join input")
                self.assertEqual(
                    self.sitegen["status_grid"]["perf_pattern"].format(pond=name),
                    expected,
                    "Status cards must resolve the same per-pond projection",
                )

    def test_perf_membership_has_no_missing_or_orphaned_stages(self):
        expected = self.instances | {"_self"}
        ingested = {
            path.removeprefix("/system/etc/measure/")
            for path, node in self.nodes.items()
            if path.startswith("/system/etc/measure/")
            and node.get("factory") == "logfile-ingest"
        } - {"limits"}
        projected = {name.removeprefix("p-") for name in self.derived if name.startswith("p-")}
        joined = {item["scope"] for item in self.inputs}
        # Fully wired entries may precede deployment or retain retired history.
        self.assertTrue(expected <= ingested, f"Missing configured ponds: {expected - ingested}")
        self.assertEqual(projected, ingested, "Ingest and projection inventories differ")
        self.assertEqual(joined, ingested, "Ingest and join inventories differ")


if __name__ == "__main__":
    unittest.main()
