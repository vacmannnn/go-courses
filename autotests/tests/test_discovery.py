"""Discovery must select the current homework and reject ambiguous layouts."""
import importlib.util
import json
import os
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

    def test_pr_titles(self):
        for title in ("3", "03", "ДЗ 3", "ДЗ №3: HTTP API", "Homework 3", "task3",
                      "task-3 — HTTP API", "task_03", "Решение task3", "task3-fix"):
            with self.subTest(title=title):
                self.assertEqual(ci.detect_task(title), "task3")

    def test_pr_title_without_task_fails(self):
        with self.assertRaisesRegex(ValueError, "по заголовку PR"):
            ci.detect_task("feature/movies")

    def test_ambiguous_pr_title_fails(self):
        for title in ("task1/task3", "ДЗ 1 и 3", "task1 и ДЗ 3"):
            with self.subTest(title=title), self.assertRaises(ValueError):
                ci.detect_task(title)

    def test_repeated_homework_number_is_unambiguous(self):
        self.assertEqual(ci.detect_task("ДЗ 3 (task3)"), "task3")

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
                                 "--pr-title", "task3", "--report", str(report)],
                                capture_output=True, text=True, timeout=30)
        self.assertNotEqual(result.returncode, 0)
        self.assertEqual(json.loads(report.read_text())["status"], "fail")

    def test_branch_number_is_not_used_when_title_has_no_task(self):
        report = self.repo / "report.json"
        env = dict(os.environ, GITHUB_HEAD_REF="task1", GITHUB_REF_NAME="task1",
                   COURSE_PR_TITLE="Movie service")
        result = subprocess.run(["python3", str(ROOT / "scripts/ci.py"),
                                 "--repo", str(ROOT / "testdata/task1-good"),
                                 "--report", str(report)], env=env,
                                capture_output=True, text=True, timeout=30)
        self.assertNotEqual(result.returncode, 0)
        data = json.loads(report.read_text())
        self.assertEqual(data["pr_title"], "Movie service")
        self.assertIn("по заголовку PR", data["log"])

    def test_automatic_grading_end_to_end(self):
        result = subprocess.run(["python3", str(ROOT / "scripts/ci.py"),
                                 "--repo", str(ROOT / "testdata/task1-good"), "--pr-title", "ДЗ 1"],
                                env=dict(os.environ, GITHUB_HEAD_REF="task3", GITHUB_REF_NAME="task3"),
                                capture_output=True, text=True, timeout=180)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("PASS: task1", result.stdout)


if __name__ == "__main__":
    unittest.main()
