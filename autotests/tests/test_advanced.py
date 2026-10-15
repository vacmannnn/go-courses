"""Tests for acceptance assertions; Docker integration requires a running daemon in CI."""
import contextlib
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "scripts"))
import advanced
import ci


class AdvancedTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()

    def write(self, path, content):
        file = self.root / path
        file.parent.mkdir(parents=True, exist_ok=True)
        file.write_text(content)
        return file

    def module(self):
        self.write("go.mod", "module example.org/student\n\ngo 1.26.4\n")

    def architecture(self):
        self.module()
        for layer in ("domain", "usecase", "adapter", "handler"):
            self.write(layer + "/types.go", "package " + layer + "\ntype Movie struct { ID int }\n")
        self.write("cmd/server/main.go", "package main\nfunc main() {}\n")

    def test_all_ten_numeric_pr_titles(self):
        for number in range(1, 11):
            self.assertEqual(ci.detect_task(str(number)), "task" + str(number))
            self.assertEqual(ci.detect_task("task" + str(number)), "task" + str(number))

    def test_does_not_extract_arbitrary_issue_number(self):
        with self.assertRaises(ValueError):
            ci.detect_task("Fix issue3")

    def test_numbered_module_selection(self):
        for number in (1, 2, 3):
            self.write(str(number) + "/go.mod", "module student\n\ngo 1.26.4\n")
        self.assertEqual(ci.detect_module(self.root, "task3"), self.root / "3")

    def test_architecture_layers_present(self):
        self.architecture()
        advanced.architecture([self.root])

    def test_architecture_rejects_transport_in_usecase(self):
        self.architecture()
        self.write("usecase/types.go", 'package usecase\nimport "net/http"\nvar Request = http.MethodGet\n')
        with self.assertRaisesRegex(AssertionError, "transport"):
            advanced.architecture([self.root])

    def test_architecture_requires_all_layers(self):
        self.module()
        self.write("main.go", "package main\nfunc main() {}\n")
        with self.assertRaisesRegex(AssertionError, "layer"):
            advanced.architecture([self.root])

    def test_student_coverage_uses_student_tests(self):
        self.module()
        self.write("value.go", "package student\nfunc Value() int { return 42 }\n")
        self.write("value_test.go", 'package student\nimport "testing"\nfunc TestValue(t *testing.T) { if Value()!=42 {t.Fatal("bad")} }\n')
        message = advanced.student_tests([self.root], dict(os.environ, GOWORK="off"), coverage=True)
        self.assertIn("100.00%", message)

    def test_coverage_without_student_tests_fails(self):
        self.module()
        self.write("value.go", "package student\nfunc Value() int { return 42 }\n")
        with self.assertRaisesRegex(AssertionError, ">70%"):
            advanced.student_tests([self.root], dict(os.environ, GOWORK="off"), coverage=True)

    def test_upstream_movies_and_failure(self):
        upstream = advanced.Upstream()
        self.addCleanup(upstream.close)
        for number, movie in enumerate(advanced.MOVIES, 1):
            status, _, body = advanced.http(upstream.url + "/{}/info.0.json".format(number))
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body), movie)
        self.assertEqual(advanced.http(upstream.url + "/4/info.0.json")[0], 404)
        upstream.offline = True
        self.assertEqual(advanced.http(upstream.url + "/1/info.0.json")[0], 500)

    def test_docker_failure_is_red_not_skipped(self):
        self.module()
        self.write("main.go", "package main\nfunc main() {}\n")
        with mock.patch.object(advanced, "Docker", side_effect=AssertionError("Docker unavailable")):
            code, output = advanced.evaluate("task5", self.root)
        self.assertEqual(code, 1)
        events = [json.loads(line) for line in output.splitlines()]
        self.assertTrue(any(event["Action"] == "fail" for event in events))
        self.assertFalse(any(event["Action"] == "skip" for event in events))

    def k8s(self):
        documents = [{"kind": "ConfigMap"}, {"kind": "Secret"}]
        for role in ("catalog", "downloader"):
            documents += [{"apiVersion": "apps/v1", "kind": "Deployment", "metadata": {"name": role + "-service"},
                           "spec": {"replicas": 2, "selector": {"matchLabels": {"app": role}},
                                    "template": {"metadata": {"labels": {"app": role}}, "spec": {"containers": [
                                        {"name": role, "image": role, "readinessProbe": {"tcpSocket": {"port": 8080}},
                                         "livenessProbe": {"tcpSocket": {"port": 8080}},
                                         "resources": {"requests": {"cpu": "100m", "memory": "128Mi"},
                                                       "limits": {"cpu": "1", "memory": "256Mi"}}}]}}}},
                          {"kind": "Service", "metadata": {"name": role}, "spec": {"selector": {"app": role}}}]
        return documents

    def test_k8s_accepts_probes_resources_and_selectors(self):
        self.write("k8s/resources.yaml", json.dumps({"kind": "List", "items": self.k8s()}))
        advanced.kubernetes(self.root)

    def test_k8s_rejects_single_catalog_replica(self):
        resources = self.k8s()
        resources[2]["spec"]["replicas"] = 1
        self.write("k8s/resources.yaml", json.dumps({"kind": "List", "items": resources}))
        with self.assertRaisesRegex(AssertionError, "multiple replicas"):
            advanced.kubernetes(self.root)

    def test_k8s_rejects_missing_probe(self):
        resources = self.k8s()
        del resources[2]["spec"]["template"]["spec"]["containers"][0]["readinessProbe"]
        self.write("k8s/resources.yaml", json.dumps({"kind": "List", "items": resources}))
        with self.assertRaisesRegex(AssertionError, "probes"):
            advanced.kubernetes(self.root)

    def test_compose_file_is_required(self):
        with self.assertRaisesRegex(AssertionError, "Compose file"):
            advanced.compose_file(self.root)

    def observability(self):
        for index in range(6):
            self.write("grafana/{}.json".format(index), json.dumps({"title": "Dashboard " + str(index), "panels": [{"type": "timeseries"}]}))
        self.write("grafana/providers.yaml", json.dumps({"providers": [{"name": "course"}]}))
        self.write("grafana/datasources.yaml", json.dumps({"datasources": [{"name": "Prometheus"}]}))
        return {"services": {name: {"image": name} for name in ("prometheus", "grafana", "loki", "alloy")}}

    def test_observability_needs_six_dashboards(self):
        config = self.observability()
        advanced.observability_config(self.root, config)
        (self.root / "grafana/5.json").unlink()
        with self.assertRaisesRegex(AssertionError, "six"):
            advanced.observability_config(self.root, config)

    def test_observability_rejects_missing_log_collector(self):
        config = self.observability()
        del config["services"]["alloy"]
        with self.assertRaisesRegex(AssertionError, "log collector"):
            advanced.observability_config(self.root, config)

    def test_metrics_and_structured_logs(self):
        body = "# TYPE http_requests_total counter\n" + "\n".join(name + " 1" for name in (
            "http_requests_total", "movie_search_total", "redis_requests_total", "kafka_messages_total", "postgres_queries_total"))
        with mock.patch.object(advanced, "http", return_value=(200, {}, body)):
            advanced.metrics("http://example.invalid", '{"level":"INFO","msg":"request","request_id":"abc"}')
            with self.assertRaisesRegex(AssertionError, "structured"):
                advanced.metrics("http://example.invalid", "plain text")

    def test_missing_suite_number_fails(self):
        result = subprocess.run([sys.executable, str(ROOT / "scripts/check.py"), "11", str(self.root)], capture_output=True)
        self.assertNotEqual(result.returncode, 0)


if __name__ == "__main__":
    unittest.main()
