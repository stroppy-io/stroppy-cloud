import copy
import hashlib
import json
import tempfile
import unittest
from pathlib import Path

import live


class LedgerTests(unittest.TestCase):
    def test_aggregation_never_promotes_unknown_or_unrun_checks(self):
        def group(*states):
            return live.aggregate([{"status": state} for state in states])

        self.assertEqual(group("passed", "not_applicable"), "passed 1/1")
        self.assertEqual(group("passed", "unknown"), "unknown 1/2")
        self.assertEqual(group("passed", "not_run"), "not_run 1/2")
        self.assertEqual(group("failed", "unknown"), "failed 0/2")
        self.assertEqual(group("not_applicable"), "not_applicable")
        self.assertEqual(group(), "unknown 0/0")

    def test_reference_checks_pointer_and_rejects_escape(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / "proof.json").write_text('{"cells":[{"status":"passed"}]}')
            live.reference(root, {"path": "proof.json", "pointer": "/cells/0/status"})
            with self.assertRaises((KeyError, IndexError)):
                live.reference(root, {"path": "proof.json", "pointer": "/cells/9"})
            with self.assertRaises(ValueError):
                live.inside(root, "../outside.json")

    def test_all_historical_rows_survive_with_their_identity_and_run(self):
        old = live.read(live.ROOT / "tests/platform/migration/legacy-progress.json")
        migrated = [case["legacy_row"] for _, case, _ in live.cases(live.ROOT) if "legacy_row" in case]
        canonical = lambda rows: sorted(json.dumps(row, sort_keys=True) for row in rows)
        self.assertEqual(canonical(old), canonical(migrated))

    def test_drafts_have_no_success_or_executed_attempt(self):
        drafts = [item for item in live.cases(live.ROOT) if item[1]["execution"]["status"] == "not_started"]
        for _, case, record in drafts:
            self.assertIsNone(case["selected_run"])
            self.assertEqual(case["runs"], [])
            self.assertFalse(any(check["status"] == "passed" for check in record["checks"]))

    def test_shared_runs_point_to_the_same_payload(self):
        owners = {}
        aliases = 0
        for path, case, _ in live.cases(live.ROOT):
            for name in case["runs"]:
                run = path.parent / name
                resolved = run.resolve()
                rid = Path(name).name
                if rid in owners:
                    self.assertEqual(owners[rid], resolved)
                owners[rid] = resolved
                aliases += run.is_symlink()
        self.assertGreaterEqual(len(owners), 67)
        self.assertGreater(aliases, 0)

    def test_preserved_tools_match_inventory_without_executing_them(self):
        root = live.ROOT / "tools/history"
        inventory = live.read(root / "inventory.json")
        self.assertGreaterEqual(len(inventory), 232)
        for entry in inventory:
            path = root / entry["path"]
            self.assertEqual(hashlib.sha256(path.read_bytes()).hexdigest(), entry["sha256"], str(path))

    def test_current_ledger_validates(self):
        self.assertGreaterEqual(live.validate(live.ROOT), 270)

    def test_passed_without_evidence_is_rejected(self):
        # Isolate one draft: no external historical payloads or live endpoints.
        _, source, source_checks = next(item for item in live.cases(live.ROOT) if item[1]["selected_run"] is None)
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            case = copy.deepcopy(source)
            record = copy.deepcopy(source_checks)
            case["source"] = {"path": "proof.json"}
            record["checks"][0].update(status="passed", evidence=None)
            for name, value in {
                case["id"] + "/case.json": case,
                case["id"] + "/checks.json": record,
                case["id"] + "/input.json": {},
                "proof.json": {},
            }.items():
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(json.dumps(value))
            with self.assertRaisesRegex(ValueError, "Passed without evidence"):
                live.validate(root)


if __name__ == "__main__":
    unittest.main()
