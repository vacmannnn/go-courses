#!/usr/bin/env python3
"""Detect the homework and Go entry point, then run its acceptance tests."""
import argparse
import json
import os
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parent.parent
SUPPORTED = {"task" + str(number) for number in range(1, 11)}
TASK_PATTERN = re.compile(r"(?:^|[^a-z0-9])task[-_ ]?0*([1-9][0-9]*)(?=$|[^a-z0-9])", re.I)
IGNORED = {"vendor", "node_modules", "testdata", "reports", "__pycache__"}


def tasks_in(value):
    tasks = {"task" + match for match in TASK_PATTERN.findall(value)}
    tasks.update("task" + str(int(part)) for part in value.split("/") if re.fullmatch(r"0*[1-9][0-9]*", part))
    return tasks


def detect_task(title):
    tasks = {"task" + match for match in TASK_PATTERN.findall(title)}
    tasks.update("task" + str(int(number))
                 for number in re.findall(r"(?<!\w)0*([1-9][0-9]*)(?!\w)", title))
    if len(tasks) != 1:
        raise ValueError("Не удалось определить одно ДЗ по заголовку PR {!r}. "
                         "Укажите в заголовке один номер ДЗ: например, 'ДЗ 3' или 'task3'.".format(title))
    task = tasks.pop()
    if task not in SUPPORTED:
        raise ValueError("Для {} тесты пока не реализованы. Проверка не может считаться успешной.".format(task))
    return task


def inside(root, relative):
    if Path(relative).is_absolute():
        raise ValueError("Путь должен быть относительным: " + relative)
    result = (root / relative).resolve()
    if not result.is_relative_to(root):
        raise ValueError("Путь выходит за пределы решения: " + relative)
    return result


def detect_module(repo, task):
    modules = []
    for directory, names, files in os.walk(repo, followlinks=False):
        names[:] = sorted(name for name in names if not name.startswith(".") and
                          name not in IGNORED and not (Path(directory) / name).is_symlink())
        path = Path(directory)
        labels = tasks_in(path.relative_to(repo).as_posix())
        if labels and labels != {task}:
            names[:] = []
            continue
        if "go.mod" in files and not (path / "go.mod").is_symlink():
            modules.append(path)
    matching = [path for path in modules if task in tasks_in(path.relative_to(repo).as_posix())]
    candidates = matching or ([repo] if repo in modules else modules)
    if len(candidates) != 1:
        found = ", ".join(path.relative_to(repo).as_posix() for path in candidates) or "не найдены"
        raise ValueError("Не удалось выбрать один Go-модуль для {} (go.mod: {}). "
                         "Держите решение в корне или в папке {} с одним go.mod.".format(task, found, task))
    return candidates[0]


def decode_packages(output):
    decoder = json.JSONDecoder()
    while output.strip():
        package, offset = decoder.raw_decode(output.lstrip())
        yield package
        output = output.lstrip()[offset:]


def detect_package(module, task):
    env = os.environ.copy()
    env["GOWORK"] = "off"
    env.pop("GOOS", None)
    env.pop("GOARCH", None)
    result = subprocess.run(["go", "list", "-e", "-json", "./..."], cwd=module,
                            env=env, capture_output=True, text=True, timeout=180)
    candidates = []
    try:
        packages = list(decode_packages(result.stdout))
    except json.JSONDecodeError as exc:
        raise ValueError("Не удалось прочитать результат go list: " + result.stderr) from exc
    for package in packages:
        if package.get("Name") != "main" or not package.get("GoFiles"):
            continue
        path = Path(package["Dir"]).resolve()
        if not path.is_relative_to(module):
            continue
        relative = path.relative_to(module)
        if any(part in IGNORED or part.startswith(".") for part in relative.parts):
            continue
        labels = tasks_in(relative.as_posix())
        if labels and labels != {task}:
            continue
        candidates.append(relative.as_posix())
    matching = [path for path in candidates if task in tasks_in(path)]
    candidates = matching or candidates
    if len(candidates) != 1:
        found = ", ".join(candidates) or "не найдены"
        raise ValueError("Не удалось выбрать один пакет main ({}). "
                         "В решении должен быть один запускаемый main-пакет. {}".format(found, result.stderr.strip()))
    return candidates[0]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", required=True, type=Path)
    parser.add_argument("--pr-title", default=os.environ.get("COURSE_PR_TITLE", ""))
    parser.add_argument("--task", default="auto")
    parser.add_argument("--module", default="auto")
    parser.add_argument("--package", default="auto")
    parser.add_argument("--report", type=Path)
    args = parser.parse_args()
    try:
        repo = args.repo.expanduser().resolve()
        if not repo.is_dir():
            raise ValueError("Репозиторий решения не найден: " + str(repo))
        task = detect_task(args.pr_title) if args.task == "auto" else detect_task(args.task)
        if task not in SUPPORTED:
            raise ValueError("Тесты для {} пока не реализованы".format(task))
        if args.module == "auto" and int(task[4:]) >= 7:
            candidates = [repo / task, repo / task[4:]]
            module = next((path for path in candidates if path.is_dir()), repo)
        else:
            module = detect_module(repo, task) if args.module == "auto" else inside(repo, args.module)
        if int(task[4:]) < 7 and not (module / "go.mod").is_file():
            raise ValueError("В выбранном модуле нет go.mod: " + str(module))
        package = (detect_package(module, task) if int(task[4:]) < 4 else ".") if args.package == "auto" else args.package
        inside(module, package)
    except (OSError, ValueError, subprocess.TimeoutExpired) as exc:
        message = "Ошибка определения решения: " + str(exc)
        print(message, file=sys.stderr)
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps({"status": "fail", "stage": "discovery", "pr_title": args.pr_title,
                                               "tests": [], "log": message}, ensure_ascii=False, indent=2) + "\n")
        return 1
    print("ДЗ: {} | модуль: {} | пакет main: {}".format(task, module.relative_to(repo), package), flush=True)
    command = [sys.executable, str(ROOT / "scripts/check.py"), task, str(module), "--package", package]
    if args.report:
        command.extend(["--report", str(args.report)])
    return subprocess.call(command)


if __name__ == "__main__":
    sys.exit(main())
