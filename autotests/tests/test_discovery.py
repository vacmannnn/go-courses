"""Discovery must select the current homework and reject ambiguous layouts."""
import importlib.util
import json
from pathlib import Path
import subprocess
import tempfile
import unittest

ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("course_ci", ROOT / "scripts/ci.py")
ci = importlib.util.module_from_spec(spec)
spec.loader.exec_module(ci)


class DiscoveryTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name).resolve()

    def module(self, path=".", package="."):
        module = self.repo / path
        module.mkdir(parents=True, exist_ok=True)
        (module / "go.mod").write_text("module example.org/student\n\ngo 1.26.4\n")
        main = module / package
        main.mkdir(parents=True, exist_ok=True)
        (main / "main.go").write_text("package main\nfunc main() {}\n")
        return module

    def test_branch_names(self):
        for branch in ("3", "03", "feature/3", "task3", "task-3", "task_03", "feature/task3", "task3-fix"):
            self.assertEqual(ci.detect_task(branch), "task3")

    def test_branch_without_task_fails(self):
        with self.assertRaisesRegex(ValueError, "по ветке"):
            ci.detect_task("feature/movies")

    def test_ambiguous_branch_fails(self):
        with self.assertRaises(ValueError):
            ci.detect_task("task1/task3")

    def test_unimplemented_task_fails(self):
        with self.assertRaisesRegex(ValueError, "пока не реализованы"):
            ci.detect_task("11")

    def test_root_module_and_main(self):
        self.module()
        self.assertEqual(ci.detect_module(self.repo, "task3"), self.repo)
        self.assertEqual(ci.detect_package(self.repo, "task3"), ".")

    def test_cmd_server(self):
        module = self.module("my project", "cmd/server")
        self.assertEqual(ci.detect_module(self.repo, "task3"), module)
        self.assertEqual(ci.detect_package(module, "task3"), "cmd/server")

    def test_current_task_module_wins_over_previous_homework(self):
        self.module("task1")
        self.module("task2")
        current = self.module("task3", "cmd/server")
        self.assertEqual(ci.detect_module(self.repo, "task3"), current)
        self.assertEqual(ci.detect_package(current, "task3"), "cmd/server")

    def test_current_task_module_wins_over_root(self):
        self.module()
        current = self.module("task3")
        self.assertEqual(ci.detect_module(self.repo, "task3"), current)

    def test_old_main_packages_are_ignored(self):
        self.module(package="task2")
        current = self.repo / "task3"
        current.mkdir()
        (current / "main.go").write_text("package main\nfunc main() {}\n")
        self.assertEqual(ci.detect_package(self.repo, "task3"), "task3")

    def test_multiple_main_packages_fail(self):
        self.module(package="cmd/server")
        client = self.repo / "cmd/client"
        client.mkdir()
        (client / "main.go").write_text("package main\nfunc main() {}\n")
        with self.assertRaisesRegex(ValueError, "один пакет main"):
            ci.detect_package(self.repo, "task3")

    def test_multiple_unlabelled_modules_fail(self):
        self.module("one")
        self.module("two")
        with self.assertRaisesRegex(ValueError, "один Go-модуль"):
            ci.detect_module(self.repo, "task3")

    def test_missing_module_fails(self):
        with self.assertRaisesRegex(ValueError, "не найдены"):
            ci.detect_module(self.repo, "task3")

    def test_paths_cannot_escape_checkout(self):
        with self.assertRaises(ValueError):
            ci.inside(self.repo, "../other")

    def test_build_tags_respected(self):
        self.module()
        other = self.repo / "tools"
        other.mkdir()
        (other / "main.go").write_text("//go:build never_enabled_course_fixture\n\npackage main\nfunc main() {}\n")
        self.assertEqual(ci.detect_package(self.repo, "task3"), ".")

    def test_discovery_failure_produces_red_report(self):
        report = self.repo / "report.json"
        result = subprocess.run(["python3", str(ROOT / "scripts/ci.py"), "--repo", str(self.repo),
                                 "--branch", "task3", "--report", str(report)],
                                capture_output=True, text=True, timeout=30)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(json.loads(report.read_text())["status"], "fail")

    def test_automatic_grading_end_to_end(self):
        result = subprocess.run(["python3", str(ROOT / "scripts/ci.py"),
                                 "--repo", str(ROOT / "testdata/task1-good"), "--branch", "task1"],
                                capture_output=True, text=True, timeout=180)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("PASS: task1", result.stdout)


if __name__ == "__main__":
    unittest.main()
