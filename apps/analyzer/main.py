#!/usr/bin/env python
import argparse
import json
import math
import os
import re
import subprocess
import sys
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple


STACK_LINE_RE = re.compile(r"^\s*[0-9a-fA-F]+\s+(.+?)\s+\(([^()]*)\)\s*$")
SAMPLE_LINE_RE = re.compile(r"^\S.*:\s+\S+:\s*$")
PERF_SAMPLE_TIME_RE = re.compile(r"^\S.*\s(?P<timestamp>\d+(?:\.\d+)?):\s+\S+:\s*$")
BPFTRACE_MAP_RE = re.compile(r"^@\[(?P<key>.+?)\]:\s*(?P<count>\d+)\s*$")


def parse_args(argv: Optional[List[str]] = None) -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Mini-Drop analyzer")
    parser.add_argument("--task-id", required=True)
    parser.add_argument("--raw-path", required=True)
    parser.add_argument("--output-dir", required=True)
    parser.add_argument("--target-pid", required=True, type=int)
    parser.add_argument("--sample-rate", required=True, type=int)
    parser.add_argument("--sample-duration", required=True, type=int)
    return parser.parse_args(argv)


def main(argv: Optional[List[str]] = None) -> int:
    args = parse_args(argv)
    raw_path = Path(args.raw_path)
    output_dir = Path(args.output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    started = time.perf_counter()

    log_event(
        "analyzer_started",
        task_id=args.task_id,
        target_pid=args.target_pid,
        raw_path=str(raw_path),
        output_dir=str(output_dir),
        sample_rate_hz=args.sample_rate,
        sample_duration_sec=args.sample_duration,
    )

    if not raw_path.exists():
        message = f"raw artifact not found: {raw_path}"
        log_event("analyzer_failed", task_id=args.task_id, error=message, duration_ms=elapsed_ms(started))
        print(json.dumps({"error": message}), file=sys.stderr)
        return 1

    try:
        analysis = analyze_raw_artifact(args, raw_path, output_dir)
    except Exception as exc:
        log_event("analyzer_failed", task_id=args.task_id, error=str(exc), duration_ms=elapsed_ms(started))
        print(json.dumps({"error": str(exc)}), file=sys.stderr)
        return 1

    flamegraph_path = output_dir / "flamegraph.svg"
    topn_path = output_dir / "topn.json"
    timeline_path = output_dir / "resource_timeline.json"
    collapsed_path = output_dir / "collapsed.txt"

    collapsed_text = render_collapsed_stacks(analysis.collapsed) if analysis.collapsed else ""
    if collapsed_text:
        collapsed_path.write_text(collapsed_text, encoding="utf-8")
    render_flamegraph(
        flamegraph_path=flamegraph_path,
        task_id=args.task_id,
        target_pid=args.target_pid,
        sample_rate=args.sample_rate,
        sample_duration=args.sample_duration,
        hotspots=analysis.hotspots,
        collapsed=analysis.collapsed,
        collapsed_text=collapsed_text,
        title=analysis.title,
        subtitle=analysis.subtitle,
    )
    topn_path.write_text(json.dumps(analysis.hotspots, indent=2), encoding="utf-8")
    timeline_path.write_text(json.dumps(analysis.resource_timeline, indent=2), encoding="utf-8")

    result = {
        "flamegraph_path": to_relative_artifact_path(flamegraph_path),
        "topn_path": to_relative_artifact_path(topn_path),
        "resource_timeline_path": to_relative_artifact_path(timeline_path),
        "summary": analysis.summary,
    }
    log_event(
        "analyzer_completed",
        task_id=args.task_id,
        hotspot_count=len(analysis.hotspots),
        flamegraph_path=result["flamegraph_path"],
        topn_path=result["topn_path"],
        duration_ms=elapsed_ms(started),
    )
    print(json.dumps(result))
    return 0


def log_event(event: str, **fields: Any) -> None:
    payload = {
        "time": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "level": "INFO",
        "component": "analyzer",
        "event": event,
    }
    payload.update(fields)
    if "error" in fields:
        payload["level"] = "ERROR"
    print(json.dumps(payload, ensure_ascii=True, sort_keys=True), file=sys.stderr)


def elapsed_ms(started: float) -> int:
    return int((time.perf_counter() - started) * 1000)


class Analysis:
    def __init__(
        self,
        *,
        hotspots: List[Dict[str, Any]],
        collapsed: Dict[Tuple[str, ...], int],
        title: str,
        subtitle: str,
        summary: str,
        resource_timeline: Dict[str, Any],
    ) -> None:
        self.hotspots = hotspots
        self.collapsed = collapsed
        self.title = title
        self.subtitle = subtitle
        self.summary = summary
        self.resource_timeline = resource_timeline


@dataclass
class FlameNode:
    name: str
    value: int = 0
    children: Dict[str, "FlameNode"] = field(default_factory=dict)


def analyze_raw_artifact(args: argparse.Namespace, raw_path: Path, output_dir: Path) -> Analysis:
    if raw_path.suffix == ".json":
        return analyze_mock_json(args, raw_path)
    if raw_path.name == "ebpf.syscalls.txt":
        return analyze_ebpf_syscalls(args, raw_path)
    if raw_path.name == "pyspy.raw.txt":
        return analyze_pyspy_raw(args, raw_path)
    return analyze_perf_data(args, raw_path, output_dir)


def analyze_mock_json(args: argparse.Namespace, raw_path: Path) -> Analysis:
    raw_payload = json.loads(raw_path.read_text(encoding="utf-8"))
    frames = raw_payload.get("frames", [])
    collapsed = collapsed_from_mock_frames(frames)
    hotspots = build_hotspots_from_collapsed(collapsed)
    return Analysis(
        hotspots=hotspots,
        collapsed=collapsed,
        title="Mini-Drop Mock Flamegraph",
        subtitle="Synthetic profile generated by the analyzer CLI to exercise the full platform flow.",
        summary=f"Mock profile for PID {args.target_pid} captured at {args.sample_rate}Hz for {args.sample_duration}s.",
        resource_timeline=build_resource_timeline(
            args=args,
            hotspots=hotspots,
            source="derived_from_mock_profile",
            signal="cpu_hotspot",
            alignment="cpu",
        ),
    )


def analyze_perf_data(args: argparse.Namespace, raw_path: Path, output_dir: Path) -> Analysis:
    script_path = output_dir / "perf.script.txt"
    script_text = run_perf_script(raw_path)
    script_path.write_text(script_text, encoding="utf-8")

    collapsed = collapse_perf_script(script_text, output_dir)
    if not collapsed:
        raise RuntimeError("perf script produced no stack samples")

    hotspots = build_hotspots_from_collapsed(collapsed)
    return Analysis(
        hotspots=hotspots,
        collapsed=collapsed,
        title="Mini-Drop Perf Flamegraph",
        subtitle="Profile generated from perf.data via perf script and the built-in stack parser.",
        summary=f"Perf profile for PID {args.target_pid} captured at {args.sample_rate}Hz for {args.sample_duration}s.",
        resource_timeline=build_resource_timeline(
            args=args,
            hotspots=hotspots,
            source="perf_script_samples",
            signal="cpu_cycles",
            alignment="cpu",
            sample_times=parse_perf_sample_times(script_text),
        ),
    )


def collapse_perf_script(script_text: str, output_dir: Path) -> Dict[Tuple[str, ...], int]:
    external = try_stackcollapse_perf(script_text, output_dir)
    if external is not None:
        return external
    return parse_perf_script(script_text)


def analyze_ebpf_syscalls(args: argparse.Namespace, raw_path: Path) -> Analysis:
    counts = parse_bpftrace_syscalls(raw_path.read_text(encoding="utf-8"))
    if not counts:
        raise RuntimeError("bpftrace syscall artifact produced no counters")

    collapsed = {("syscalls", syscall): count for syscall, count in counts.items()}
    hotspots = build_hotspots_from_counts(counts)
    return Analysis(
        hotspots=hotspots,
        collapsed=collapsed,
        title="Mini-Drop eBPF Syscall Distribution",
        subtitle="Syscall histogram collected with bpftrace tracepoints.",
        summary=f"eBPF syscall profile for PID {args.target_pid} captured for {args.sample_duration}s.",
        resource_timeline=build_resource_timeline(
            args=args,
            hotspots=hotspots,
            source="derived_from_ebpf_syscall_counts",
            signal="syscall_pressure",
            alignment="syscall",
        ),
    )


def analyze_pyspy_raw(args: argparse.Namespace, raw_path: Path) -> Analysis:
    collapsed = parse_collapsed_stacks(raw_path.read_text(encoding="utf-8"))
    if not collapsed:
        raise RuntimeError("py-spy raw artifact produced no stack samples")

    hotspots = build_hotspots_from_collapsed(collapsed)
    return Analysis(
        hotspots=hotspots,
        collapsed=collapsed,
        title="Mini-Drop py-spy Flamegraph",
        subtitle="Python user-space stacks collected with py-spy raw format.",
        summary=f"py-spy profile for PID {args.target_pid} captured at {args.sample_rate}Hz for {args.sample_duration}s.",
        resource_timeline=build_resource_timeline(
            args=args,
            hotspots=hotspots,
            source="derived_from_pyspy_stacks",
            signal="python_cpu",
            alignment="userspace/python",
        ),
    )


def run_perf_script(raw_path: Path) -> str:
    try:
        completed = subprocess.run(
            ["perf", "script", "-i", str(raw_path)],
            check=False,
            capture_output=True,
            text=True,
            timeout=60,
        )
    except FileNotFoundError as exc:
        raise RuntimeError("perf command not found while analyzing perf.data") from exc
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("perf script timeout after 60s") from exc

    if completed.returncode != 0:
        details = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
        raise RuntimeError(f"perf script failed: {details}")
    return completed.stdout


def try_stackcollapse_perf(script_text: str, output_dir: Path) -> Optional[Dict[Tuple[str, ...], int]]:
    stackcollapse = os.environ.get("MINIDROP_STACKCOLLAPSE_PERF", "").strip()
    if not stackcollapse:
        return None

    script_path = Path(stackcollapse)
    if not script_path.exists():
        raise RuntimeError(f"stackcollapse-perf.pl not found: {script_path}")

    collapsed_path = output_dir / "stackcollapse-perf.txt"
    try:
        completed = subprocess.run(
            [str(script_path)],
            input=script_text,
            check=False,
            capture_output=True,
            text=True,
            timeout=60,
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("stackcollapse-perf.pl timeout after 60s") from exc
    except OSError as exc:
        raise RuntimeError(f"cannot execute stackcollapse-perf.pl: {exc}") from exc

    if completed.returncode != 0:
        details = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
        raise RuntimeError(f"stackcollapse-perf.pl failed: {details}")

    collapsed_path.write_text(completed.stdout, encoding="utf-8")
    parsed = parse_collapsed_stacks(completed.stdout)
    if parsed:
        return parsed

    print("stackcollapse-perf.pl produced no collapsed stacks; falling back to built-in parser", file=sys.stderr)
    return None


def collapsed_from_mock_frames(frames: List[Dict[str, Any]]) -> Dict[Tuple[str, ...], int]:
    collapsed: Dict[Tuple[str, ...], int] = {}
    for frame in frames:
        samples = int(frame.get("samples", 0))
        stack = tuple(str(item) for item in (frame.get("stack") or []) if str(item).strip())
        if not stack or samples <= 0:
            continue
        collapsed[stack] = collapsed.get(stack, 0) + samples
    return collapsed


def parse_perf_script(script_text: str) -> Dict[Tuple[str, ...], int]:
    collapsed: Dict[Tuple[str, ...], int] = {}
    current_stack: List[str] = []

    for line in script_text.splitlines():
        stripped = line.rstrip()
        if not stripped:
            flush_stack(collapsed, current_stack)
            current_stack = []
            continue

        if SAMPLE_LINE_RE.match(stripped):
            flush_stack(collapsed, current_stack)
            current_stack = []
            continue

        match = STACK_LINE_RE.match(stripped)
        if not match:
            continue

        symbol = clean_symbol(match.group(1))
        if symbol:
            current_stack.append(symbol)

    flush_stack(collapsed, current_stack)
    return collapsed


def parse_perf_sample_times(script_text: str) -> List[float]:
    timestamps: List[float] = []
    for line in script_text.splitlines():
        match = PERF_SAMPLE_TIME_RE.match(line.rstrip())
        if not match:
            continue
        try:
            timestamps.append(float(match.group("timestamp")))
        except ValueError:
            continue
    return timestamps


def build_resource_timeline(
    *,
    args: argparse.Namespace,
    hotspots: List[Dict[str, Any]],
    source: str,
    signal: str,
    alignment: str,
    sample_times: Optional[List[float]] = None,
) -> Dict[str, Any]:
    duration = max(int(args.sample_duration), 1)
    top = hotspots[0] if hotspots else {"function": "unknown", "samples": 0, "percent": 0.0}
    points = build_timeline_points(duration, top, sample_times or [])
    peak = max((float(point["value"]) for point in points), default=0.0)
    summary = (
        f"{source} timeline aligned {alignment} across {duration}s; "
        f"top={top['function']} peak={peak:.1f}%"
    )
    return {
        "source": source,
        "signal": signal,
        "alignment": alignment,
        "summary": summary,
        "window_sec": duration,
        "top_function": str(top["function"]),
        "peak_percent": round(peak, 1),
        "points": points,
    }


def build_timeline_points(duration: int, top: Dict[str, Any], sample_times: List[float]) -> List[Dict[str, Any]]:
    bucket_count = min(max(duration, 1), 12)
    bucket_width = duration / bucket_count
    if sample_times:
        return points_from_sample_times(sample_times, bucket_count, bucket_width)
    return derived_profile_points(top, bucket_count, bucket_width)


def points_from_sample_times(sample_times: List[float], bucket_count: int, bucket_width: float) -> List[Dict[str, Any]]:
    first = min(sample_times)
    counts = [0 for _ in range(bucket_count)]
    for timestamp in sample_times:
        offset = max(timestamp - first, 0.0)
        index = int(offset / bucket_width) if bucket_width > 0 else 0
        if index >= bucket_count:
            index = bucket_count - 1
        counts[index] += 1

    max_count = max(counts) if counts else 0
    points: List[Dict[str, Any]] = []
    for index, count in enumerate(counts):
        value = round((count / max_count) * 100 if max_count else 0.0, 1)
        points.append(
            {
                "offset_sec": round(index * bucket_width, 1),
                "value": value,
                "samples": count,
            }
        )
    return points


def derived_profile_points(top: Dict[str, Any], bucket_count: int, bucket_width: float) -> List[Dict[str, Any]]:
    percent = float(top.get("percent", 0.0))
    samples = int(top.get("samples", 0))
    points: List[Dict[str, Any]] = []
    center = (bucket_count - 1) / 2 if bucket_count > 1 else 0.0
    for index in range(bucket_count):
        distance = abs(index - center) / max(center, 1)
        value = max(percent * (1 - distance * 0.45), min(percent, 8.0))
        points.append(
            {
                "offset_sec": round(index * bucket_width, 1),
                "value": round(value, 1),
                "samples": max(1, round(samples / max(bucket_count, 1))) if samples > 0 else 0,
            }
        )
    return points


def flush_stack(collapsed: Dict[Tuple[str, ...], int], stack: List[str]) -> None:
    if not stack:
        return
    ordered = tuple(reversed(stack))
    collapsed[ordered] = collapsed.get(ordered, 0) + 1


def clean_symbol(symbol: str) -> str:
    symbol = symbol.strip()
    symbol = re.sub(r"\+0x[0-9a-fA-F]+.*$", "", symbol)
    symbol = symbol.replace(";", ":")
    return symbol or "[unknown]"


def render_collapsed_stacks(collapsed: Dict[Tuple[str, ...], int]) -> str:
    lines = []
    for stack, samples in sorted(collapsed.items(), key=lambda item: item[1], reverse=True):
        lines.append(f"{';'.join(stack)} {samples}")
    return "\n".join(lines) + ("\n" if lines else "")


def parse_collapsed_stacks(text: str) -> Dict[Tuple[str, ...], int]:
    collapsed: Dict[Tuple[str, ...], int] = {}
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        stack_part, _, count_part = stripped.rpartition(" ")
        if not stack_part or not count_part:
            continue
        try:
            samples = int(count_part)
        except ValueError:
            continue
        if samples <= 0:
            continue

        stack = tuple(frame.strip().replace(";", ":") for frame in stack_part.split(";") if frame.strip())
        if not stack:
            continue
        collapsed[stack] = collapsed.get(stack, 0) + samples
    return collapsed


def build_hotspots_from_collapsed(collapsed: Dict[Tuple[str, ...], int]) -> List[Dict[str, Any]]:
    totals: Dict[str, int] = {}
    total_samples = 0
    for stack, samples in collapsed.items():
        if not stack:
            continue
        leaf = stack[-1]
        totals[leaf] = totals.get(leaf, 0) + samples
        total_samples += samples

    hotspots = []
    for function, samples in sorted(totals.items(), key=lambda item: item[1], reverse=True):
        percent = round((samples / total_samples) * 100 if total_samples else 0, 1)
        hotspots.append({"function": function, "samples": samples, "percent": percent})
    return hotspots


def parse_bpftrace_syscalls(text: str) -> Dict[str, int]:
    counts: Dict[str, int] = {}
    for line in text.splitlines():
        stripped = line.strip()
        if not stripped or stripped.startswith("#"):
            continue

        match = BPFTRACE_MAP_RE.match(stripped)
        if not match:
            continue

        syscall = clean_bpftrace_key(match.group("key"))
        if not syscall:
            continue
        count = int(match.group("count"))
        if count <= 0:
            continue
        counts[syscall] = counts.get(syscall, 0) + count

    return dict(sorted(counts.items(), key=lambda item: item[1], reverse=True))


def clean_bpftrace_key(key: str) -> str:
    key = key.strip().strip('"')
    key = key.replace("tracepoint:syscalls:sys_enter_", "")
    key = key.replace("tracepoint:syscalls:", "")
    key = key.replace("sys_enter_", "")
    key = key.replace(";", ":")
    return key.strip() or "[unknown]"


def build_hotspots_from_counts(counts: Dict[str, int]) -> List[Dict[str, Any]]:
    total = sum(count for count in counts.values() if count > 0)
    hotspots: List[Dict[str, Any]] = []
    for name, count in sorted(counts.items(), key=lambda item: item[1], reverse=True):
        if count <= 0:
            continue
        percent = round((count / total) * 100 if total else 0, 1)
        hotspots.append({"function": name, "samples": count, "percent": percent})
    return hotspots


def render_flamegraph(
    *,
    flamegraph_path: Path,
    task_id: str,
    target_pid: int,
    sample_rate: int,
    sample_duration: int,
    hotspots: List[Dict[str, Any]],
    collapsed: Dict[Tuple[str, ...], int],
    collapsed_text: str,
    title: str,
    subtitle: str,
) -> None:
    flamegraph_script = os.environ.get("MINIDROP_FLAMEGRAPH_PL", "").strip()
    if flamegraph_script and collapsed_text:
        try:
            rendered = run_flamegraph_pl(Path(flamegraph_script), collapsed_text, title)
        except RuntimeError as exc:
            print(f"flamegraph.pl fallback: {exc}", file=sys.stderr)
        else:
            flamegraph_path.write_text(rendered, encoding="utf-8")
            return

    flamegraph_path.write_text(
        render_flamegraph_svg(
            task_id=task_id,
            target_pid=target_pid,
            sample_rate=sample_rate,
            sample_duration=sample_duration,
            hotspots=hotspots,
            collapsed=collapsed,
            title=title,
            subtitle=subtitle,
        ),
        encoding="utf-8",
    )


def run_flamegraph_pl(script_path: Path, collapsed_text: str, title: str) -> str:
    if not script_path.exists():
        raise RuntimeError(f"{script_path} does not exist")

    try:
        completed = subprocess.run(
            [str(script_path), "--title", title],
            input=collapsed_text,
            check=False,
            capture_output=True,
            text=True,
            timeout=60,
        )
    except subprocess.TimeoutExpired as exc:
        raise RuntimeError("flamegraph.pl timeout after 60s") from exc
    except OSError as exc:
        raise RuntimeError(f"cannot execute flamegraph.pl: {exc}") from exc

    if completed.returncode != 0:
        details = completed.stderr.strip() or completed.stdout.strip() or f"exit code {completed.returncode}"
        raise RuntimeError(details)
    if "<svg" not in completed.stdout:
        raise RuntimeError("flamegraph.pl output did not contain SVG")
    return completed.stdout


def render_flamegraph_svg(
    task_id: str,
    target_pid: int,
    sample_rate: int,
    sample_duration: int,
    hotspots: List[Dict[str, Any]],
    collapsed: Dict[Tuple[str, ...], int],
    title: str,
    subtitle: str,
) -> str:
    width = 1200
    chart_left = 56
    chart_top = 184
    chart_width = 1088
    frame_height = 24
    frame_gap = 2

    root = flame_tree_from_collapsed(collapsed) if collapsed else flame_tree_from_hotspots(hotspots)
    max_depth = max_flame_depth(root)
    height = max(420, chart_top + (max_depth + 2) * (frame_height + frame_gap) + 84)
    frames = render_flame_frames(root, chart_left, chart_top, chart_width, frame_height, frame_gap)
    total_samples = sum(int(item["samples"]) for item in hotspots)

    return f"""<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {width} {height}" role="img" aria-labelledby="title desc">
  <title id="title">{escape_xml(title)} for {escape_xml(task_id)}</title>
  <desc id="desc">Analysis result for PID {target_pid}</desc>
  <defs>
    <linearGradient id="bg" x1="0%" y1="0%" x2="100%" y2="100%">
      <stop offset="0%" stop-color="#08111f" />
      <stop offset="100%" stop-color="#111827" />
    </linearGradient>
  </defs>
  <rect width="{width}" height="{height}" fill="url(#bg)" />
  <text x="60" y="64" font-size="34" fill="#f8fafc" font-family="Georgia, 'Times New Roman', serif">{escape_xml(title)}</text>
  <text x="60" y="102" font-size="18" fill="#94a3b8" font-family="Verdana, Geneva, sans-serif">Task {escape_xml(task_id)} | PID {target_pid} | {sample_rate}Hz | {sample_duration}s</text>
  <text x="60" y="144" font-size="14" fill="#64748b" font-family="Verdana, Geneva, sans-serif">{escape_xml(subtitle)}</text>
  <text x="60" y="166" font-size="13" fill="#94a3b8" font-family="Verdana, Geneva, sans-serif">Total samples: {total_samples}</text>
  {"".join(frames)}
</svg>
"""


def flame_tree_from_hotspots(hotspots: List[Dict[str, Any]]) -> FlameNode:
    root = FlameNode("root")
    for item in hotspots:
        samples = int(item["samples"])
        name = str(item["function"])
        root.value += samples
        node = root.children.get(name)
        if node is None:
            node = FlameNode(name)
            root.children[name] = node
        node.value += samples
    return root


def flame_tree_from_collapsed(collapsed: Dict[Tuple[str, ...], int]) -> FlameNode:
    root = FlameNode("root")
    for stack, samples in collapsed.items():
        if not stack or samples <= 0:
            continue
        root.value += samples
        current = root
        for frame_name in stack:
            child = current.children.get(frame_name)
            if child is None:
                child = FlameNode(frame_name)
                current.children[frame_name] = child
            child.value += samples
            current = child
    return root


def max_flame_depth(node: FlameNode) -> int:
    if not node.children:
        return 0
    return 1 + max(max_flame_depth(child) for child in node.children.values())


def render_flame_frames(
    root: FlameNode,
    x: float,
    y: float,
    width: float,
    frame_height: int,
    frame_gap: int,
) -> List[str]:
    if root.value <= 0:
        return []
    frames: List[str] = []
    render_node_children(root, x, y, width, frame_height, frame_gap, 0, frames)
    return frames


def render_node_children(
    node: FlameNode,
    x: float,
    y: float,
    width: float,
    frame_height: int,
    frame_gap: int,
    depth: int,
    out: List[str],
) -> None:
    cursor = x
    child_y = y + depth * (frame_height + frame_gap)
    children = sorted(node.children.values(), key=lambda child: child.value, reverse=True)
    for index, child in enumerate(children):
        child_width = width * (child.value / node.value)
        if child_width < 1:
            continue
        out.append(render_single_frame(child, cursor, child_y, child_width, frame_height, depth, index))
        render_node_children(child, cursor, y, child_width, frame_height, frame_gap, depth + 1, out)
        cursor += child_width


def render_single_frame(node: FlameNode, x: float, y: float, width: float, height: int, depth: int, index: int) -> str:
    color = flame_color(node.name, depth, index)
    label = truncate_label(node.name, width)
    text = ""
    if label:
        text = f'<text x="{x + 7:.1f}" y="{y + 16:.1f}" font-size="12" fill="#111827" font-family="Verdana, Geneva, sans-serif">{escape_xml(label)}</text>'
    return f"""
  <g>
    <title>{escape_xml(node.name)} ({node.value} samples)</title>
    <rect x="{x:.1f}" y="{y:.1f}" width="{max(width - 1, 0):.1f}" height="{height}" rx="2" ry="2" fill="{color}" stroke="#0f172a" stroke-width="0.4"/>
    {text}
  </g>
"""


def flame_color(name: str, depth: int, index: int) -> str:
    seed = sum(ord(ch) for ch in name) + depth * 31 + index * 17
    hue = 24 + (seed % 44)
    saturation = 74 + (seed % 16)
    lightness = 54 + (seed % 10)
    return f"hsl({hue}, {saturation}%, {lightness}%)"


def truncate_label(label: str, width: float) -> str:
    max_chars = int((width - 14) / 7)
    if max_chars < 4:
        return ""
    if len(label) <= max_chars:
        return label
    return label[: max_chars - 3] + "..."


def to_relative_artifact_path(path: Path) -> str:
    parts = path.parts
    if "artifacts" in parts:
        idx = parts.index("artifacts")
        return "/".join(parts[idx + 1 :])
    return path.as_posix()


def escape_xml(text: str) -> str:
    return (
        text.replace("&", "&amp;")
        .replace("<", "&lt;")
        .replace(">", "&gt;")
        .replace('"', "&quot;")
        .replace("'", "&apos;")
    )


def main_with_argv(argv: List[str]) -> int:
    return main(argv)


if __name__ == "__main__":
    raise SystemExit(main())
