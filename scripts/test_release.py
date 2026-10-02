"""Exercise tag-only release behavior against a disposable local Git remote."""
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("release.sh").resolve()


class ReleaseTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="stroppy-release-test-")
        self.addCleanup(self.temp.cleanup)
        root = Path(self.temp.name)
        self.remote = root / "remote.git"
        self.repo = root / "repo"
        self.run_git("init", "--bare", str(self.remote), cwd=root)
        self.run_git("init", "-b", "main", str(self.repo), cwd=root)
        self.run_git("config", "user.name", "Release Test")
        self.run_git("config", "user.email", "release-test@example.invalid")
        (self.repo / "file").write_text("initial\n")
        self.run_git("add", "file")
        self.run_git("commit", "-m", "initial")
        self.run_git("remote", "add", "origin", str(self.remote))
        self.run_git("push", "-u", "origin", "main")
        self.env = dict(os.environ)
        traps = root / "traps"
        traps.mkdir()
        for name in ("go", "yarn", "npm", "docker", "make"):
            command = traps / name
            command.write_text("#!/bin/sh\necho 'unexpected build command' >&2\nexit 99\n")
            command.chmod(0o755)
        self.env["PATH"] = str(traps) + os.pathsep + self.env["PATH"]

    def run_git(self, *args, cwd=None):
        return subprocess.check_output(
            ["git", *args], cwd=cwd or self.repo, stderr=subprocess.PIPE, text=True
        ).strip()

    def release(self, version="0.1.0", answer="yes\n"):
        return subprocess.run(
            ["bash", str(SCRIPT), version], input=answer, text=True,
            cwd=self.repo, env=self.env, capture_output=True, check=False,
        )

    def test_pushes_annotated_tag_without_building(self):
        result = self.release()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.run_git("cat-file", "-t", "v0.1.0"), "tag")
        remote_commit = self.run_git("rev-parse", "v0.1.0^{commit}", cwd=self.remote)
        self.assertEqual(remote_commit, self.run_git("rev-parse", "HEAD"))

    def test_refuses_dirty_tree(self):
        (self.repo / "file").write_text("unfinished\n")
        self.assertNotEqual(self.release().returncode, 0)
        self.assertEqual(self.run_git("tag", "-l", cwd=self.remote), "")

    def test_refuses_unpushed_commit(self):
        self.run_git("commit", "--allow-empty", "-m", "not pushed")
        self.assertNotEqual(self.release().returncode, 0)
        self.assertEqual(self.run_git("tag", "-l", cwd=self.remote), "")

    def test_cancel_does_not_tag(self):
        self.assertEqual(self.release(answer="no\n").returncode, 0)
        self.assertEqual(self.run_git("tag", "-l"), "")

    def test_refuses_existing_and_older_versions(self):
        self.assertEqual(self.release().returncode, 0)
        for version in ("0.1.0", "0.0.9", "01.2.3", "0.2.0-rc.1"):
            with self.subTest(version=version):
                self.assertNotEqual(self.release(version).returncode, 0)
        self.assertEqual(self.run_git("tag", "-l", cwd=self.remote), "v0.1.0")

    def test_interactive_patch_ignores_legacy_development_tags(self):
        self.run_git("tag", "0.0.0-dev7")
        self.assertEqual(self.release().returncode, 0)
        result = self.release(version="", answer="3\nyes\n")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.run_git("cat-file", "-t", "v0.1.1", cwd=self.remote), "tag")


if __name__ == "__main__":
    unittest.main()
