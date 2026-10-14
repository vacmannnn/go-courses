"""Acceptance suites 4–10. Container checks fail (never skip) if Docker is unavailable."""
import contextlib
import html.parser
import json
import os
from pathlib import Path
import re
import shutil
import socket
import subprocess
import tempfile
import threading
import time
import urllib.error
import urllib.parse
import urllib.request
import uuid
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

ROOT = Path(__file__).resolve().parent.parent
MOVIES = json.loads((ROOT / "fixtures/movies.json").read_text())


def require(condition, message):
    if not condition:
        raise AssertionError(message)


def run(args, cwd=None, env=None, timeout=180, input=None):
    result = subprocess.run([str(arg) for arg in args], cwd=cwd, env=env, input=input,
                            capture_output=True, text=True, timeout=timeout)
    require(result.returncode == 0, "{} failed:\n{}\n{}".format(" ".join(map(str, args)), result.stdout, result.stderr))
    return result.stdout


def eventually(action, timeout=30):
    end = time.monotonic() + timeout
    last = None
    while time.monotonic() < end:
        try:
            result = action()
            if result:
                return result
        except (AssertionError, OSError, ValueError) as exc:
            last = exc
        time.sleep(0.15)
    raise AssertionError("Timed out waiting for a condition: {}".format(last))


def http(url):
    try:
        response = urllib.request.urlopen(url, timeout=3)
    except urllib.error.HTTPError as exc:
        response = exc
    with response:
        return response.status, response.headers, response.read(2**20).decode(errors="replace")


def films(base, path="/load-movies?from=1&to=3"):
    status, _, body = http(base + path)
    require(status == 200, "Expected HTTP 200 at {}: {} {}".format(path, status, body))
    data = json.loads(body)
    require(isinstance(data, list), "Expected a JSON array of movies")
    return data


def expect_catalog(base):
    data = films(base)
    require(sorted(movie.get("id") for movie in data) == [1, 2, 3], "Expected exactly movie ids 1,2,3: " + repr(data))
    for movie in data:
        want = MOVIES[movie["id"] - 1]
        for field in ("title", "year", "director"):
            require(movie.get(field) == want[field], "Wrong {} for movie {}".format(field, movie["id"]))
    return True


def free_port():
    with socket.socket() as sock:
        sock.bind(("127.0.0.1", 0))
        return sock.getsockname()[1]


class Upstream:
    def __init__(self):
        self.offline = False
        self.requests = []
        self.lock = threading.Lock()
        upstream = self

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                match = re.fullmatch(r"/([1-9][0-9]*)/info\.0\.json", self.path)
                with upstream.lock:
                    upstream.requests.append(self.path)
                if upstream.offline:
                    self.send_error(500)
                    return
                if not match or int(match[1]) > len(MOVIES):
                    self.send_error(404)
                    return
                body = json.dumps(MOVIES[int(match[1]) - 1]).encode()
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, *args):
                pass

        self.server = ThreadingHTTPServer(("0.0.0.0", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)
        self.thread.start()
        self.port = self.server.server_port
        self.url = "http://127.0.0.1:" + str(self.port)

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        self.thread.join(timeout=3)


def list_modules(source, task):
    from ci import tasks_in
    modules = []
    for directory, dirs, files in os.walk(source):
        dirs[:] = [name for name in dirs if name not in {".git", "vendor", "node_modules", "testdata", "reports"}
                   and not (Path(directory) / name).is_symlink()]
        path = Path(directory)
        labels = tasks_in(path.relative_to(source).as_posix())
        if labels and labels != {task}:
            dirs[:] = []
            continue
        if "go.mod" in files:
            modules.append(path)
    require(modules, "No Go modules found in the homework")
    return modules


def packages(module):
    from ci import decode_packages
    env = os.environ.copy()
    env["GOWORK"] = "off"
    return list(decode_packages(run(["go", "list", "-e", "-json", "./..."], cwd=module, env=env)))


def architecture(modules):
    layers = {name: [] for name in ("domain", "usecase", "adapter", "handler", "cmd")}
    for module in modules:
        for package in packages(module):
            directory = Path(package.get("Dir", module))
            if not directory.is_relative_to(module):
                continue
            parts = directory.relative_to(module).parts
            for layer in layers:
                if layer in parts:
                    layers[layer].append(package)
    for layer, entries in layers.items():
        require(entries, "Architecture: no compiled Go packages in layer " + layer)
    for layer in ("domain", "usecase"):
        for package in layers[layer]:
            for imported in package.get("Imports", []):
                forbidden = imported == "net/http" or imported.startswith("net/http/") or imported.startswith("google.golang.org/grpc")
                forbidden |= any(part in {"adapter", "handler", "cmd"} for part in imported.split("/"))
                if layer == "domain":
                    forbidden |= "usecase" in imported.split("/")
                require(not forbidden, "{} imports external layer/transport {}".format(package.get("ImportPath"), imported))


class Docker:
    def __init__(self):
        self.containers = []
        self.env = {}
        self.pg = self.redis = self.kafka = None
        run(["docker", "info", "--format", "{{.ServerVersion}}"], timeout=15)

    def start(self, image, port, environment=None, command=None, fixed_port=None):
        name = "course-tests-" + uuid.uuid4().hex[:12]
        self.containers.append(name)
        args = ["docker", "run", "-d", "--name", name, "--label", "go-course-tests=true",
                "-p", "127.0.0.1:{}:{}".format(fixed_port or "", port)]
        for key, value in (environment or {}).items():
            args += ["-e", key + "=" + value]
        run(args + [image] + (command or []), timeout=240)
        hostport = int(run(["docker", "port", name, str(port) + "/tcp"]).strip().rsplit(":", 1)[1])
        return name, hostport

    def exec(self, container, *args, input=None):
        return run(["docker", "exec", "-i", container] + list(args), input=input, timeout=30)

    def services(self, number):
        self.pg, port = self.start("postgres:16-alpine", 5432,
                                   {"POSTGRES_DB": "movies", "POSTGRES_USER": "course", "POSTGRES_PASSWORD": "course"})
        eventually(lambda: self.exec(self.pg, "pg_isready", "-U", "course", "-d", "movies"), timeout=60)
        self.env["DATABASE_URL"] = "postgres://course:course@127.0.0.1:{}/movies?sslmode=disable".format(port)
        if number >= 6:
            self.redis, port = self.start("redis:7-alpine", 6379)
            eventually(lambda: self.exec(self.redis, "redis-cli", "PING").strip() == "PONG")
            self.env["REDIS_ADDR"] = "127.0.0.1:" + str(port)
        if number >= 7:
            port = free_port()
            self.kafka, _ = self.start("docker.redpanda.com/redpandadata/redpanda:v26.2.4", port,
                command=["redpanda", "start", "--mode", "dev-container", "--smp", "1",
                         "--kafka-addr", "0.0.0.0:" + str(port), "--advertise-kafka-addr", "127.0.0.1:" + str(port)], fixed_port=port)
            self.brokers = "127.0.0.1:" + str(port)
            eventually(lambda: self.rpk("cluster", "info"), timeout=90)
            for topic in ("movie_download_tasks", "movie_downloaded"):
                self.rpk("topic", "create", topic)
            self.env["KAFKA_BROKERS"] = self.brokers

    def rpk(self, *args, input=None):
        return self.exec(self.kafka, "rpk", *args, "-X", "brokers=" + self.brokers, input=input)

    def sql(self, query):
        return self.exec(self.pg, "psql", "-X", "-U", "course", "-d", "movies", "-tA", "-v", "ON_ERROR_STOP=1", "-c", query).strip()

    def stored_movies(self):
        tables = json.loads(self.sql("SELECT coalesce(json_agg(table_name), '[]') FROM information_schema.columns WHERE table_schema='public' AND column_name='title'"))
        require(tables, "PostgreSQL must contain a public movie table with a title column")
        total = 0
        for table in tables:
            quoted = '"' + table.replace('"', '""') + '"'
            total += int(self.sql("SELECT count(*) FROM " + quoted + " WHERE title IN ('Solaris','Spirited Away','The Grand Budapest Hotel')"))
        return total

    def redis_ttls(self):
        keys = self.exec(self.redis, "redis-cli", "--scan").splitlines()
        return {key: int(self.exec(self.redis, "redis-cli", "TTL", key)) for key in keys}

    def close(self):
        for name in reversed(self.containers):
            subprocess.run(["docker", "rm", "-f", "-v", name], capture_output=True, timeout=30)


class Apps:
    def __init__(self, workspace, modules, number, env, upstream):
        self.workspace, self.number, self.env, self.upstream = workspace, number, env, upstream
        self.processes, self.entries = [], []
        self.logfiles = []
        for module in modules:
            for package in packages(module):
                if package.get("Name") == "main" and package.get("GoFiles"):
                    directory = Path(package["Dir"])
                    if directory.is_relative_to(module):
                        from ci import tasks_in
                        labels = tasks_in(directory.relative_to(workspace).as_posix())
                        if labels and labels != {"task" + str(number)}:
                            continue
                        self.entries.append((module, directory))
        if number < 7:
            require(len(self.entries) == 1, "Expected one main package, found " + repr(self.entries))
            self.catalog, self.downloader = self.entries[0], None
        else:
            self.catalog = self.find("catalog")
            self.downloader = self.find("downloader")
        self.binaries = {}
        for entry in self.entries:
            binary = workspace / ("app-" + uuid.uuid4().hex)
            run(["go", "build", "-o", binary, "./" + entry[1].relative_to(entry[0]).as_posix()], cwd=entry[0], env=env)
            self.binaries[entry] = binary

    def find(self, role):
        matches = [entry for entry in self.entries if any(role in part.lower() for part in entry[1].relative_to(self.workspace).parts)]
        require(len(matches) == 1, "Expected one {} main package, found {}".format(role, matches))
        return matches[0]

    def start(self):
        port = free_port()
        self.base = "http://127.0.0.1:" + str(port)
        env = dict(self.env, HTTP_ADDR="127.0.0.1:" + str(port), MOVIE_BASE_URL=self.upstream.url)
        # Downloader is ready to consume before the catalog publishes startup work.
        order = ([self.downloader] if self.downloader else []) + [self.catalog]
        for entry in order:
            logfile = tempfile.TemporaryFile(mode="w+t")
            self.logfiles.append(logfile)
            process = subprocess.Popen([str(self.binaries[entry])], cwd=entry[0], env=env,
                                       stdout=logfile, stderr=subprocess.STDOUT)
            self.processes.append(process)
        try:
            eventually(lambda: expect_catalog(self.base), timeout=45)
        except AssertionError as exc:
            raise AssertionError(str(exc) + "\nApplication logs:\n" + self.logs()) from exc
        return self.base

    def logs(self):
        text = []
        for file in self.logfiles:
            file.seek(0)
            text.append(file.read())
        return "\n".join(text)

    def stop(self):
        for process in self.processes:
            if process.poll() is None:
                process.terminate()
        for process in self.processes:
            try:
                process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait(timeout=5)
        self.processes.clear()

    def close(self):
        self.stop()
        for file in self.logfiles:
            file.close()


def search(base):
    # One word at a time: does not assume AND/OR semantics or language stemming.
    for term, movie_id in (("Solaris", 1), ("psychologist", 1), ("Tarkovsky", 1), ("Banionis", 1), ("animation", 2)):
        results = films(base, "/search?" + urllib.parse.urlencode({"q": term}))
        require(movie_id in [movie.get("id") for movie in results], "Search for {!r} did not return movie {}".format(term, movie_id))
    require(films(base, "/search?q=unmatched-course-query-7291") == [], "Unknown term should return an empty array")


def cache(base, docker):
    before = docker.redis_ttls()
    first = films(base, "/search?q=Solaris")
    second = films(base, "/search?q=Solaris")
    require(first == second, "Repeated search returned different results")
    ttls = docker.redis_ttls()
    require(any(3500 <= ttl <= 3600 for ttl in ttls.values()), "No Redis cache entry with a one-hour TTL: " + repr(ttls))
    require(ttls or before, "Redis is empty")
    # The cached response must remain available while PostgreSQL is temporarily stopped.
    try:
        run(["docker", "stop", docker.pg], timeout=30)
        require(films(base, "/search?q=Solaris") == first, "Cache hit should not query PostgreSQL")
    finally:
        run(["docker", "start", docker.pg], timeout=30)
        eventually(lambda: docker.exec(docker.pg, "pg_isready", "-U", "course", "-d", "movies"), timeout=30)


class FormParser(html.parser.HTMLParser):
    def __init__(self):
        super().__init__()
        self.action = None
        self.in_form = False
        self.query = False

    def handle_starttag(self, tag, attributes):
        attrs = dict(attributes)
        if tag == "form" and self.action is None:
            require(attrs.get("method", "get").lower() == "get", "Search form must use GET")
            self.action, self.in_form = attrs.get("action", ""), True
        if self.in_form and tag == "input" and attrs.get("name") == "q":
            self.query = True

    def handle_endtag(self, tag):
        if tag == "form":
            self.in_form = False


def web_ui(base):
    status, headers, body = http(base + "/")
    require(status == 200 and "text/html" in headers.get("Content-Type", ""), "GET / must serve the HTML search form")
    parser = FormParser()
    parser.feed(body)
    require(parser.action is not None and parser.query, "Expected a form with input name=q")
    target = urllib.parse.urljoin(base + "/", parser.action)
    require(urllib.parse.urlsplit(target).netloc == urllib.parse.urlsplit(base).netloc, "Form must submit to this service")
    target += ("&" if "?" in target else "?") + "q=Solaris"
    status, headers, body = http(target)
    require(status == 200 and "text/html" in headers.get("Content-Type", "") and "Solaris" in body,
            "HTML result page must contain the found movie")


def student_tests(modules, env, coverage=False):
    statements = covered = 0
    for module in modules:
        profile = module / "course-student-coverage.out"
        args = ["go", "test", "-count=1", "-timeout=120s", "./..."]
        if coverage:
            args[2:2] = ["-coverpkg=./...", "-coverprofile=" + str(profile)]
        run(args, cwd=module, env=env, timeout=180)
        if coverage:
            for line in profile.read_text().splitlines()[1:]:
                _, count, hits = line.rsplit(" ", 2)
                statements += int(count)
                covered += int(count) if int(hits) > 0 else 0
    if coverage:
        ratio = 100 * covered / statements if statements else 0
        require(statements > 0 and ratio > 70, "Student coverage must be >70%, got {:.2f}% ({}/{})".format(ratio, covered, statements))
        return "Student statement coverage: {:.2f}%".format(ratio)
    return "Student go test ./... passed"


def yaml_documents(source):
    import yaml
    for path in sorted(source.rglob("*")):
        if path.suffix.lower() not in {".yaml", ".yml"} or any(part in {"vendor", ".git", "testdata"} for part in path.parts):
            continue
        for document in yaml.safe_load_all(path.read_text()):
            if isinstance(document, dict):
                if document.get("kind") == "List":
                    yield from document.get("items", [])
                else:
                    yield document


def kubernetes(source):
    documents = list(yaml_documents(source))
    kinds = {document.get("kind") for document in documents}
    require({"Deployment", "Service", "ConfigMap", "Secret"} <= kinds, "Kubernetes: need Deployment, Service, ConfigMap and Secret")
    deployments = [d for d in documents if d.get("kind") == "Deployment"]
    services = [d for d in documents if d.get("kind") == "Service"]
    for role in ("catalog", "downloader"):
        matches = [d for d in deployments if role in d.get("metadata", {}).get("name", "")]
        require(len(matches) == 1, "Expected one Kubernetes Deployment for " + role)
        deployment = matches[0]
        spec = deployment.get("spec", {})
        labels = spec.get("template", {}).get("metadata", {}).get("labels", {})
        selector = spec.get("selector", {}).get("matchLabels", {})
        require(selector and all(labels.get(k) == v for k, v in selector.items()), "Deployment selector must match pod labels")
        require(any(s.get("spec", {}).get("selector") and all(labels.get(k) == v for k, v in s["spec"]["selector"].items()) for s in services), "No Service selecting " + role)
        containers = spec.get("template", {}).get("spec", {}).get("containers", [])
        require(containers, "Deployment has no containers")
        for container in containers:
            require(container.get("readinessProbe") and container.get("livenessProbe"), "Missing probes for " + role)
            for resource in ("requests", "limits"):
                value = container.get("resources", {}).get(resource, {})
                require(value.get("cpu") and value.get("memory"), "Missing CPU/memory " + resource)
        if role == "catalog":
            require(spec.get("replicas", 1) >= 2, "catalog Deployment must demonstrate multiple replicas")


def compose_file(source):
    candidates = [source / name for name in ("compose.yaml", "compose.yml", "docker-compose.yml", "docker-compose.yaml")]
    candidates = [path for path in candidates if path.is_file()]
    require(len(candidates) == 1, "Expected one Compose file in homework root, found " + repr(candidates))
    return candidates[0]


class Compose:
    def __init__(self, source, upstream):
        self.source, self.project = source, "course-" + uuid.uuid4().hex[:12]
        self.file = compose_file(source)
        self.prefix = ["docker", "compose", "-p", self.project, "-f", str(self.file)]
        run(["docker", "info", "--format", "{{.ServerVersion}}"], timeout=15)
        self.config = json.loads(run(self.prefix + ["config", "--format", "json"], cwd=source))
        self.roles = {}
        services = self.config.get("services", {})
        for role in ("catalog", "downloader"):
            names = [name for name in services if role in name]
            require(len(names) == 1, "Compose must have one " + role + " service")
            self.roles[role] = names[0]
            require(services[names[0]].get("build"), role + " must be built from its Dockerfile")
        images = " ".join(str(service.get("image", "")) for service in services.values()).lower()
        for dependency in ("postgres", "redis"):
            require(dependency in images, "Compose lacks " + dependency)
        require("kafka" in images or "redpanda" in images, "Compose lacks Kafka")
        # Only override the fixture URL; application ports and infrastructure come from the student's Compose.
        override = {"services": {name: {"environment": {"MOVIE_BASE_URL": "http://host.docker.internal:" + str(upstream.port)},
                                         "extra_hosts": ["host.docker.internal:host-gateway"]}
                                  for name in self.roles.values()}}
        self.override = source / "course-compose-override.json"
        self.override.write_text(json.dumps(override))
        self.prefix += ["-f", str(self.override)]

    def start(self):
        run(self.prefix + ["up", "-d", "--build", "--wait", "--wait-timeout", "120"], cwd=self.source, timeout=600)
        ports = self.config["services"][self.roles["catalog"]].get("ports", [])
        require(ports, "catalog must publish its HTTP port in Compose")
        for port in ports:
            target = port.get("target") if isinstance(port, dict) else str(port).rsplit(":", 1)[-1]
            published = run(self.prefix + ["port", self.roles["catalog"], str(target)], cwd=self.source).strip()
            hostport = published.rsplit(":", 1)[-1]
            base = "http://127.0.0.1:" + hostport
            try:
                eventually(lambda: expect_catalog(base), timeout=20)
                self.base = base
                return base
            except (AssertionError, OSError, ValueError):
                continue
        raise AssertionError("No published catalog port served the movie HTTP API")

    def logs(self):
        return run(self.prefix + ["logs", "--no-color", "--no-log-prefix"], cwd=self.source)

    def close(self):
        subprocess.run(self.prefix + ["down", "--volumes", "--remove-orphans"], cwd=self.source, capture_output=True, timeout=60)


def observability_config(source, config):
    images = " ".join(str(service.get("image", "")) for service in config.get("services", {}).values()).lower()
    for name in ("prometheus", "grafana", "loki"):
        require(name in images, "Compose lacks " + name)
    require("alloy" in images or "promtail" in images, "Compose lacks a log collector (Alloy/Promtail)")
    dashboards = {}
    for path in source.rglob("*.json"):
        if any(part in {"vendor", "testdata", ".git"} for part in path.parts):
            continue
        try:
            doc = json.loads(path.read_text())
        except (ValueError, OSError):
            continue
        if isinstance(doc, dict):
            doc = doc.get("dashboard", doc)
            if isinstance(doc, dict) and doc.get("title") and doc.get("panels"):
                dashboards[doc["title"]] = doc
    require(len(dashboards) >= 6, "Expected at least six distinct nonempty Grafana dashboards, got " + str(len(dashboards)))
    provisioning = list(yaml_documents(source))
    require(any(doc.get("providers") for doc in provisioning), "No Grafana dashboard provisioning providers")
    require(any(doc.get("datasources") for doc in provisioning), "No Grafana datasource provisioning")


def metrics(base, logs):
    status, _, body = http(base + "/metrics")
    require(status == 200, "GET /metrics must return 200")
    samples = re.findall(r"^([a-zA-Z_:][a-zA-Z0-9_:]*)(?:\{[^\n]*\})?\s+[-+0-9.eE]+(?:\s+[0-9]+)?$", body, re.M)
    require(samples and "# TYPE " in body, "Expected Prometheus text exposition with numeric samples and metric types")
    names = " ".join(samples).lower()
    for category in (("http", "grpc"), ("search", "movie"), ("redis",), ("kafka",), ("postgres", "pg_", "database")):
        require(any(word in names for word in category), "No metric family for " + "/".join(category))
    events = []
    for line in logs.splitlines():
        try:
            event = json.loads(line)
        except ValueError:
            continue
        if isinstance(event, dict):
            events.append(event)
    require(any(event.get("level") and event.get("msg") for event in events), "No structured slog JSON records")
    require(any(re.search(r"request[_-]?id|correlation[_-]?id", json.dumps(event), re.I) for event in events), "No request/correlation id in structured logs")


def evaluate(task, source):
    from check import copy_solution
    number = int(task[4:])
    events = []

    def case(name, action):
        start = time.monotonic()
        try:
            detail = action()
            events.append({"Action": "pass", "Test": name, "Elapsed": time.monotonic() - start,
                           "Output": "PASS " + name + (": " + detail if isinstance(detail, str) else "") + "\n"})
        except (AssertionError, OSError, ValueError, subprocess.SubprocessError, ImportError) as exc:
            events.append({"Action": "fail", "Test": name, "Output": "FAIL {}: {}\n".format(name, exc)})
            raise

    try:
        with tempfile.TemporaryDirectory(prefix="go-course-advanced-") as tmp, contextlib.ExitStack() as cleanup:
            workspace = (Path(tmp) / "solution").resolve()
            copy_solution(source, workspace)
            modules = list_modules(workspace, task)
            env = dict(os.environ, GOWORK="off")
            env.pop("GOOS", None)
            env.pop("GOARCH", None)
            case("TestCourseBuild", lambda: [run(["go", "build", "./..."], cwd=module, env=env) for module in modules])
            if number == 4:
                case("TestCourseArchitecture", lambda: architecture(modules))
                apps = Apps(workspace, modules, number, env, None)
                cleanup.callback(apps.close)
                binary = apps.binaries[apps.catalog]
                def regression():
                    from check import parse_events
                    out = run(["go", "test", "-json", "-race", "-count=1", "-timeout=60s", "./task3"], cwd=ROOT,
                              env=dict(env, COURSE_SOLUTION_BIN=str(binary)))
                    tests, _ = parse_events(out)
                    require(len([test for test in tests if "/" not in test["name"] and test["status"] == "pass"]) == 5,
                            "Expected all five HTTP regressions to run")
                case("TestCourseHTTPRegression", regression)
                case("TestCourseStudentTests", lambda: student_tests(modules, env))
            elif number <= 8:
                docker = Docker()
                cleanup.callback(docker.close)
                case("TestCourseDependencies", lambda: docker.services(number))
                env.update(docker.env)
                upstream = Upstream()
                cleanup.callback(upstream.close)
                apps = Apps(workspace, modules, number, env, upstream)
                cleanup.callback(apps.close)
                case("TestCourseStartupSync", apps.start)
                def migrations():
                    files = list(workspace.rglob("*.sql"))
                    require(any("create table" in file.read_text().lower() for file in files), "Expected SQL schema/migration files with CREATE TABLE")
                    names = docker.sql("SELECT string_agg(table_name || '.' || column_name, ',') FROM information_schema.columns WHERE table_schema='public'").lower()
                    require("actor" in names and "genre" in names, "Database schema must store actors and genres (tables or columns)")
                case("TestCourseMigrationsAndRelations", migrations)
                case("TestCoursePostgresPersistence", lambda: require(docker.stored_movies() == 3, "Expected exactly three fixture movies in PostgreSQL"))
                def offline_reads():
                    upstream.offline = True
                    with upstream.lock:
                        before = len(upstream.requests)
                    expect_catalog(apps.base)
                    status, _, body = http(apps.base + "/last-movie")
                    require(status == 200 and json.loads(body).get("id") == 3, "last-movie must read PostgreSQL")
                    with upstream.lock:
                        require(len(upstream.requests) == before, "User API called external upstream instead of PostgreSQL")
                    upstream.offline = False
                case("TestCourseAPIReadsDatabase", offline_reads)
                if number >= 6:
                    case("TestCourseSearchFields", lambda: search(apps.base))
                    case("TestCourseRedisTTL", lambda: cache(apps.base, docker))
                if number >= 7:
                    def kafka_redelivery():
                        payload = docker.rpk("topic", "consume", "movie_downloaded", "-n", "1", "-o", "start", "-f", "%v")
                        require(payload.strip(), "No movie event in movie_downloaded")
                        docker.rpk("topic", "produce", "movie_downloaded", input=payload.rstrip("\n") + "\n")
                        time.sleep(1)
                        expect_catalog(apps.base)
                        require(docker.stored_movies() == 3, "Kafka redelivery created duplicate movies")
                        require(docker.rpk("topic", "consume", "movie_download_tasks", "-n", "1", "-o", "start", "-f", "%v").strip(), "No download task in Kafka")
                    case("TestCourseKafkaFlowAndRedelivery", kafka_redelivery)
                def restart():
                    apps.stop()
                    apps.start()
                    require(docker.stored_movies() == 3, "Restart/repeated sync created duplicate movies")
                case("TestCourseIdempotentRestart", restart)
                if number == 8:
                    case("TestCourseHTMLSearch", lambda: web_ui(apps.base))
                case("TestCourseStudentCoverage" if number == 8 else "TestCourseStudentTests",
                     lambda: student_tests(modules, dict(env, MOVIE_BASE_URL=upstream.url), coverage=number == 8))
            else:
                upstream = Upstream()
                cleanup.callback(upstream.close)
                if number == 9:
                    case("TestCourseKubernetesManifests", lambda: kubernetes(workspace))
                # Construction is inside a named case so configuration failures are reported.
                stack = []
                case("TestCourseComposeConfig", lambda: stack.append(Compose(workspace, upstream)))
                compose = stack[0]
                cleanup.callback(compose.close)
                if number == 10:
                    case("TestCourseObservabilityConfig", lambda: observability_config(workspace, compose.config))
                case("TestCourseComposeBuildAndStartup", compose.start)
                case("TestCourseComposeSearch", lambda: search(compose.base))
                if number == 10:
                    case("TestCourseMetricsAndLogs", lambda: metrics(compose.base, compose.logs()))
                case("TestCourseStudentTests", lambda: student_tests(modules, env))
    except (AssertionError, OSError, ValueError, subprocess.SubprocessError, ImportError) as exc:
        if not events or events[-1]["Action"] != "fail":
            events.append({"Action": "fail", "Test": "TestCourseInfrastructure", "Output": "FAIL: " + str(exc) + "\n"})
    failed = any(event["Action"] == "fail" for event in events)
    return (1 if failed or not events else 0), "\n".join(json.dumps(event) for event in events)
