"""End-to-end checks: correct solutions pass and meaningful defects fail."""
import json
from pathlib import Path
import subprocess
import tempfile
import unittest
import shutil

ROOT = Path(__file__).resolve().parent.parent


class GraderTests(unittest.TestCase):
    def check(self, task, solution, expected):
        with tempfile.TemporaryDirectory() as tmp:
            report = Path(tmp) / "report.json"
            result = subprocess.run(["python3", str(ROOT / "scripts/check.py"), task,
                                     str(solution), "--report", str(report)],
                                    capture_output=True, text=True, timeout=240)
            self.assertEqual(result.returncode, expected, result.stdout + result.stderr)
            data = json.loads(report.read_text())
            self.assertEqual(data["status"], "pass" if expected == 0 else "fail")
            return data

    def test_task1_correct(self):
        self.check("task1", ROOT / "testdata/task1-good", 0)

    def test_task1_rejects_duplicate_index(self):
        data = self.check("task1", ROOT / "testdata/task1-bad", 1)
        self.assertTrue(any(t["name"].startswith("TestCourseTwoSum") and t["status"] == "fail"
                            for t in data["tests"]))

    def test_task2_correct(self):
        self.check("task2", ROOT / "testdata/task2-good", 0)

    def test_task2_rejects_sequential_download(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "solution"
            shutil.copytree(ROOT / "testdata/task2-good", source)
            main = source / "main.go"
            main.write_text(main.read_text().replace("i < *workers", "i < 1"))
            data = self.check("task2", source, 1)
            self.assertTrue(any(t["name"].startswith("TestCourseWorkerLimit") and t["status"] == "fail"
                                for t in data["tests"]))

    def test_task3_correct(self):
        data = self.check("task3", ROOT / "testdata/task3-good", 0)
        top_level = [t for t in data["tests"] if "/" not in t["name"]]
        self.assertEqual(len(top_level), 5)

    def test_task3_rejects_wrong_last_movie(self):
        with tempfile.TemporaryDirectory() as tmp:
            source = Path(tmp) / "solution"
            shutil.copytree(ROOT / "testdata/task3-good", source)
            main = source / "main.go"
            main.write_text(main.read_text().replace("last = m", "last = m; break"))
            data = self.check("task3", source, 1)
            self.assertTrue(any(t["name"] == "TestCourseLastMovie" and t["status"] == "fail"
                                for t in data["tests"]))

    def test_unsupported_task_rejected(self):
        result = subprocess.run(["python3", str(ROOT / "scripts/check.py"), "task11", "."],
                                capture_output=True, text=True)
        self.assertNotEqual(result.returncode, 0)

    def test_missing_module_is_failure(self):
        with tempfile.TemporaryDirectory() as tmp:
            self.check("task1", Path(tmp), 1)


if __name__ == "__main__":
    unittest.main()
