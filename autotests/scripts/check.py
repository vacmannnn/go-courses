#!/usr/bin/env python3
"""Run trusted acceptance tests against a disposable copy of a Go module."""
import argparse
import datetime
import json
import os
from pathlib import Path
import shutil
import subprocess
import sys
import tempfile

ROOT = Path(__file__).resolve().parent.parent


def run_command(args, cwd, env, timeout):
    try:
        result = subprocess.run(args, cwd=cwd, env=env, capture_output=True,
                                text=True, timeout=timeout)
        return result.returncode, result.stdout + result.stderr
    except subprocess.TimeoutExpired as exc:
        output = exc.stdout or b""
        if isinstance(output, bytes):
            output = output.decode(errors="replace")
        return 1, output + "\nERROR: check exceeded its time limit\n"


def copy_solution(source, destination):
    def ignore(directory, names):
        ignored = []
        for name in names:
            path = Path(directory) / name
            if name in {".git", "node_modules", "reports", "__pycache__"} or path.is_symlink():
                ignored.append(name)
        return ignored
    shutil.copytree(source, destination, ignore=ignore)


def evaluate(task, source, package):
    env = os.environ.copy()
    # Do not inherit an unrelated go.work or target architecture from the caller.
    env["GOWORK"] = "off"
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    env.pop("COURSE_SOLUTION_BIN", None)
    with tempfile.TemporaryDirectory(prefix="go-course-check-") as tmp:
        workspace = (Path(tmp) / "solution").resolve()
        copy_solution(source, workspace)
        selected = (workspace / package).resolve()
        if not selected.is_relative_to(workspace.resolve()) or not selected.is_dir():
            raise ValueError("--package must be an existing directory inside the solution module")
        target = "./" + selected.relative_to(workspace).as_posix()
        if task == "task1":
            for path in workspace.rglob("*_test.go"):
                path.unlink()
            (selected / "zz_course_twosum_test.go").write_text(
                (ROOT / "task1/twosum_test.go.txt").read_text())
            return run_command(["go", "test", "-json", "-race", "-count=1", "-timeout=30s",
                                "-run=^TestCourse", target], workspace, env, 180)
        binary = Path(tmp) / "student-app"
        code, output = run_command(["go", "build", "-o", str(binary), target], workspace, env, 180)
        if code:
            return code, output
        env["COURSE_SOLUTION_BIN"] = str(binary)
        return run_command(["go", "test", "-json", "-race", "-count=1", "-timeout=60s",
                            "-run=^TestCourse", "./" + task], ROOT, env, 180)


def parse_events(output):
    tests, messages = {}, []
    for line in output.splitlines():
        try:
            event = json.loads(line)
        except json.JSONDecodeError:
            messages.append(line)
            continue
        if not isinstance(event, dict):
            messages.append(line)
            continue
        if "Output" in event:
            messages.append(event["Output"].rstrip("\n"))
        if event.get("Test") and event.get("Action") in {"pass", "fail", "skip"}:
            tests[event["Test"]] = {"name": event["Test"], "status": event["Action"],
                                    "elapsed_seconds": event.get("Elapsed", 0)}
    return list(tests.values()), "\n".join(messages)


def revision(path):
    result = subprocess.run(["git", "-C", str(path), "rev-parse", "HEAD"],
                            capture_output=True, text=True)
    return result.stdout.strip() if result.returncode == 0 else None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("task", choices=["task1", "task2", "task3"])
    parser.add_argument("solution", type=Path, help="directory containing the student's go.mod")
    parser.add_argument("--package", default=".", help="main package directory relative to solution")
    parser.add_argument("--report", type=Path, help="optional JSON report path")
    args = parser.parse_args()
    source = args.solution.expanduser().resolve()
    code, output = 1, ""
    try:
        if not (source / "go.mod").is_file():
            raise ValueError("solution must contain go.mod; select the task subdirectory if necessary")
        if not shutil.which("go"):
            raise ValueError("Go is not installed or is missing from PATH")
        if Path(args.package).is_absolute():
            raise ValueError("--package must be a relative directory")
        code, output = evaluate(args.task, source, args.package)
    except (OSError, ValueError) as exc:
        output = "ERROR: " + str(exc)
    tests, log = parse_events(output)
    # A skipped/empty suite must never produce a green grading result.
    if not any(test["status"] == "pass" for test in tests) or any(test["status"] != "pass" for test in tests):
        code = 1
    status = "pass" if code == 0 else "fail"
    report = {"task": args.task, "status": status,
              "checked_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
              "solution": str(source), "solution_revision": revision(source),
              "tests_revision": revision(ROOT), "tests": tests, "log": log}
    print(log)
    print(f"\n{status.upper()}: {args.task}")
    if args.report:
        args.report.parent.mkdir(parents=True, exist_ok=True)
        args.report.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n")
        print("Report:", args.report)
    return 0 if code == 0 else 1


if __name__ == "__main__":
    sys.exit(main())
