from __future__ import annotations

import json
import math
import os
import signal
import subprocess
import time
import urllib.error
import urllib.request
from pathlib import Path


BASE_HTTP_PORT = 8090
JOIN_TIMEOUT_SECONDS = 60


def resolve_project_root() -> Path:
    project_root = Path.cwd().resolve()
    if not (project_root / "start-cluster.sh").exists():
        project_root = project_root.parent
    return project_root


def ensure_qad_binary(project_root: Path) -> Path:
    qad_binary = project_root / "qad"
    if qad_binary.exists() and os.access(qad_binary, os.X_OK):
        return qad_binary

    subprocess.run(["make", "build"], cwd=project_root, check=True)
    return qad_binary


def http_get_json(url: str, timeout: float = 2.0) -> dict | None:
    try:
        with urllib.request.urlopen(url, timeout=timeout) as resp:
            if resp.status != 200:
                return None
            return json.loads(resp.read().decode("utf-8"))
    except Exception:
        return None


def wait_for_cluster_convergence(
    expected_nodes: int,
    timeout_seconds: int = JOIN_TIMEOUT_SECONDS,
    base_http_port: int = BASE_HTTP_PORT,
) -> bool:
    ports = [base_http_port + i for i in range(expected_nodes)]
    deadline = time.time() + timeout_seconds

    while time.time() < deadline:
        all_ready = True

        for port in ports:
            data = http_get_json(f"http://127.0.0.1:{port}/cluster", timeout=1.5)
            if not data or data.get("total_nodes") != expected_nodes:
                all_ready = False
                break

        if all_ready:
            return True

        time.sleep(1.0)

    return False


def start_cluster(
    project_root: Path,
    node_count: int,
    eviction: str,
    storage_size: int,
    timeout_seconds: int = JOIN_TIMEOUT_SECONDS,
) -> subprocess.Popen:
    script = project_root / "start-cluster.sh"
    cmd = [
        "bash",
        str(script),
        str(node_count),
        "--eviction",
        eviction,
        "--storage",
        str(storage_size),
    ]
    proc = subprocess.Popen(
        cmd,
        cwd=project_root,
        stdout=subprocess.DEVNULL,
        stderr=subprocess.STDOUT,
        preexec_fn=os.setsid,
    )

    ok = wait_for_cluster_convergence(node_count, timeout_seconds=timeout_seconds)
    if not ok:
        stop_cluster(proc)
        raise RuntimeError(
            f"Cluster failed to converge for size={node_count}, eviction={eviction}"
        )

    return proc


def stop_cluster(proc: subprocess.Popen | None) -> None:
    if proc is None or proc.poll() is not None:
        return

    try:
        os.killpg(os.getpgid(proc.pid), signal.SIGTERM)
    except ProcessLookupError:
        return

    try:
        proc.wait(timeout=20)
    except subprocess.TimeoutExpired:
        try:
            os.killpg(os.getpgid(proc.pid), signal.SIGKILL)
        except ProcessLookupError:
            pass


def post_write(
    port: int,
    key: str,
    payload: bytes,
    timeout: float = 5.0,
) -> tuple[int | None, float]:
    req = urllib.request.Request(
        url=f"http://127.0.0.1:{port}/api/{key}",
        data=payload,
        method="POST",
        headers={"Content-Type": "application/octet-stream"},
    )

    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            dt_ms = (time.perf_counter() - t0) * 1000.0
            return resp.status, dt_ms
    except urllib.error.HTTPError as e:
        dt_ms = (time.perf_counter() - t0) * 1000.0
        return e.code, dt_ms
    except Exception:
        dt_ms = (time.perf_counter() - t0) * 1000.0
        return None, dt_ms


def get_read(
    port: int,
    key: str,
    timeout: float = 5.0,
) -> tuple[int | None, float, int]:
    req = urllib.request.Request(
        url=f"http://127.0.0.1:{port}/api/{key}",
        method="GET",
    )

    t0 = time.perf_counter()
    try:
        with urllib.request.urlopen(req, timeout=timeout) as resp:
            body = resp.read()
            dt_ms = (time.perf_counter() - t0) * 1000.0
            return resp.status, dt_ms, len(body)
    except urllib.error.HTTPError as e:
        dt_ms = (time.perf_counter() - t0) * 1000.0
        return e.code, dt_ms, 0
    except Exception:
        dt_ms = (time.perf_counter() - t0) * 1000.0
        return None, dt_ms, 0


def percentile(values: list[float], p: float) -> float:
    if not values:
        return float("nan")
    xs = sorted(values)
    idx = max(0, min(len(xs) - 1, math.ceil(p * len(xs)) - 1))
    return xs[idx]
