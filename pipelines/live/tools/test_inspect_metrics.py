import io
import json
import unittest
from unittest.mock import patch

from inspect_metrics import inspect


class FilesystemInventoryTests(unittest.TestCase):
    def check_rows(self, rows):
        stream = io.BytesIO(b"".join(json.dumps(row).encode() + b"\n" for row in rows))
        with patch("inspect_metrics.urllib.request.urlopen", return_value=stream):
            return inspect("run", "namespace", endpoint="http://unused", start="2026-09-15T00:00:00Z", end="2026-09-16T00:00:00Z", raw_samples=True)

    def row(self, entity, attempt="1", size=100):
        return {"metric": {"__name__": "node_filesystem_size_bytes", "graphene.entity": entity, "graphene.attempt": attempt, "mountpoint": "/data", "device": "/dev/vdb", "fstype": "ext4"}, "timestamps": [1789513467089], "values": [size]}

    def test_retry_series_do_not_create_extra_disks(self):
        rows = [self.row(str(n)) for n in range(12)] + [self.row("1", "2"), self.row("2", "2")]
        result = self.check_rows(rows)
        self.assertEqual(len(result["data_filesystems"]), 12)
        self.assertEqual(result["data_filesystem_series_count"], 14)

    def test_missing_disk_is_not_hidden_by_retry(self):
        rows = [self.row(str(n)) for n in range(11)] + [self.row("1", "2")]
        self.assertEqual(len(self.check_rows(rows)["data_filesystems"]), 11)

    def test_conflicting_disk_size_remains_visible(self):
        rows = [self.row(str(n)) for n in range(12)] + [self.row("1", "2", 200)]
        self.assertEqual(len(self.check_rows(rows)["data_filesystems"]), 13)


if __name__ == "__main__":
    unittest.main()
