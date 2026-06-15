import json
import io
import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from contextlib import redirect_stderr, redirect_stdout
from unittest import mock

ROOT = Path(__file__).resolve().parent
if str(ROOT) not in sys.path:
    sys.path.insert(0, str(ROOT))

import main


class AnalyzerTests(unittest.TestCase):
    def test_mock_json_generates_outputs(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_path = root / "artifacts" / "tsk_test" / "raw" / "mock.perf.data.json"
            output_dir = root / "artifacts" / "tsk_test" / "analysis"
            raw_path.parent.mkdir(parents=True, exist_ok=True)
            raw_path.write_text(
                json.dumps(
                    {
                        "frames": [
                            {"stack": ["root", "app.main", "hot.loop"], "samples": 10},
                            {"stack": ["root", "app.main", "io.wait"], "samples": 5},
                        ]
                    }
                ),
                encoding="utf-8",
            )

            stdout = io.StringIO()
            stderr = io.StringIO()
            with redirect_stdout(stdout), redirect_stderr(stderr):
                code = main.main_with_argv(
                    [
                        "--task-id",
                        "tsk_test",
                        "--raw-path",
                        str(raw_path),
                        "--output-dir",
                        str(output_dir),
                        "--target-pid",
                        "123",
                        "--sample-rate",
                        "99",
                        "--sample-duration",
                        "15",
                    ]
                )

            self.assertEqual(code, 0)
            result = json.loads(stdout.getvalue())
            self.assertEqual(result["flamegraph_path"], "tsk_test/analysis/flamegraph.svg")
            self.assertEqual(result["topn_path"], "tsk_test/analysis/topn.json")
            self.assertEqual(result["resource_timeline_path"], "tsk_test/analysis/resource_timeline.json")
            log_lines = [json.loads(line) for line in stderr.getvalue().splitlines() if line.strip()]
            self.assertEqual([item["event"] for item in log_lines], ["analyzer_started", "analyzer_completed"])
            self.assertTrue(all(item["component"] == "analyzer" for item in log_lines))
            self.assertTrue((output_dir / "flamegraph.svg").exists())
            hotspots = json.loads((output_dir / "topn.json").read_text(encoding="utf-8"))
            self.assertEqual(hotspots[0]["function"], "hot.loop")
            timeline = json.loads((output_dir / "resource_timeline.json").read_text(encoding="utf-8"))
            self.assertEqual(timeline["source"], "derived_from_mock_profile")
            self.assertEqual(timeline["top_function"], "hot.loop")
            self.assertGreater(len(timeline["points"]), 0)

    def test_missing_raw_artifact_logs_failure_without_stdout(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output_dir = root / "analysis"
            stdout = io.StringIO()
            stderr = io.StringIO()
            with redirect_stdout(stdout), redirect_stderr(stderr):
                code = main.main_with_argv(
                    [
                        "--task-id",
                        "tsk_missing_raw",
                        "--raw-path",
                        str(root / "missing.json"),
                        "--output-dir",
                        str(output_dir),
                        "--target-pid",
                        "123",
                        "--sample-rate",
                        "99",
                        "--sample-duration",
                        "15",
                    ]
                )

            self.assertEqual(code, 1)
            self.assertEqual(stdout.getvalue(), "")
            log_lines = [json.loads(line) for line in stderr.getvalue().splitlines() if line.strip()]
            self.assertEqual(log_lines[0]["event"], "analyzer_started")
            self.assertEqual(log_lines[1]["event"], "analyzer_failed")
            self.assertIn("raw artifact not found", log_lines[1]["error"])
            self.assertIn("raw artifact not found", log_lines[2]["error"])

    def test_perf_script_parser_generates_collapsed_stacks(self) -> None:
        script = """
python  1000 [000] 1.0: cycles:
            7d burn_cpu (/tmp/demo)
            7e worker_loop (/tmp/demo)
            7f main (/tmp/demo)

python  1000 [000] 1.1: cycles:
            7d burn_cpu (/tmp/demo)
            7e worker_loop (/tmp/demo)
            7f main (/tmp/demo)

python  1000 [000] 1.2: cycles:
            7e idle_wait (/tmp/demo)
            7f main (/tmp/demo)
"""
        collapsed = main.parse_perf_script(script)
        self.assertEqual(collapsed[("main", "worker_loop", "burn_cpu")], 2)
        self.assertEqual(collapsed[("main", "idle_wait")], 1)

        hotspots = main.build_hotspots_from_collapsed(collapsed)
        self.assertEqual(hotspots[0]["function"], "burn_cpu")
        self.assertEqual(hotspots[0]["samples"], 2)

        tree = main.flame_tree_from_collapsed(collapsed)
        self.assertEqual(tree.value, 3)
        self.assertIn("main", tree.children)

    def test_perf_data_path_uses_perf_script(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_path = root / "perf.data"
            output_dir = root / "analysis"
            raw_path.write_bytes(b"perf data placeholder")
            completed = subprocess.CompletedProcess(
                args=["perf"],
                returncode=0,
                stdout="""
python  1000 [000] 1.0: cycles:
            7d burn_cpu (/tmp/demo)
            7e worker_loop (/tmp/demo)
            7f main (/tmp/demo)
""",
                stderr="",
            )

            with mock.patch("subprocess.run", return_value=completed):
                stdout = io.StringIO()
                with redirect_stdout(stdout):
                    code = main.main_with_argv(
                        [
                            "--task-id",
                            "tsk_perf",
                            "--raw-path",
                            str(raw_path),
                            "--output-dir",
                            str(output_dir),
                            "--target-pid",
                            "123",
                            "--sample-rate",
                            "99",
                            "--sample-duration",
                            "15",
                        ]
                    )

            self.assertEqual(code, 0)
            self.assertTrue((output_dir / "perf.script.txt").exists())
            self.assertTrue((output_dir / "collapsed.txt").exists())
            hotspots = json.loads((output_dir / "topn.json").read_text(encoding="utf-8"))
            self.assertEqual(hotspots[0]["function"], "burn_cpu")
            timeline = json.loads((output_dir / "resource_timeline.json").read_text(encoding="utf-8"))
            self.assertEqual(timeline["source"], "perf_script_samples")
            self.assertEqual(timeline["signal"], "cpu_cycles")
            self.assertGreater(timeline["peak_percent"], 0)
            svg = (output_dir / "flamegraph.svg").read_text(encoding="utf-8")
            self.assertIn("rect", svg)

    def test_perf_data_can_use_stackcollapse_perf_script(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_path = root / "perf.data"
            output_dir = root / "analysis"
            stackcollapse_path = root / "stackcollapse-perf.pl"
            raw_path.write_bytes(b"perf data placeholder")
            stackcollapse_path.write_text("# fake stackcollapse script\n", encoding="utf-8")

            perf_completed = subprocess.CompletedProcess(
                args=["perf"],
                returncode=0,
                stdout="python  1000 [000] 1.0: cycles:\n            7d burn_cpu (/tmp/demo)\n",
                stderr="",
            )
            stackcollapse_completed = subprocess.CompletedProcess(
                args=[str(stackcollapse_path)],
                returncode=0,
                stdout="main;worker_loop;external_hot 7\nmain;idle_wait 2\n",
                stderr="",
            )

            previous = os.environ.get("MINIDROP_STACKCOLLAPSE_PERF")
            os.environ["MINIDROP_STACKCOLLAPSE_PERF"] = str(stackcollapse_path)
            try:
                stdout = io.StringIO()
                with mock.patch("subprocess.run", side_effect=[perf_completed, stackcollapse_completed]) as run_mock:
                    with redirect_stdout(stdout):
                        code = main.main_with_argv(
                            [
                                "--task-id",
                                "tsk_stackcollapse",
                                "--raw-path",
                                str(raw_path),
                                "--output-dir",
                                str(output_dir),
                                "--target-pid",
                                "123",
                                "--sample-rate",
                                "99",
                                "--sample-duration",
                                "15",
                            ]
                        )
                    self.assertEqual(run_mock.call_count, 2)
            finally:
                if previous is None:
                    os.environ.pop("MINIDROP_STACKCOLLAPSE_PERF", None)
                else:
                    os.environ["MINIDROP_STACKCOLLAPSE_PERF"] = previous

            self.assertEqual(code, 0)
            self.assertTrue((output_dir / "stackcollapse-perf.txt").exists())
            hotspots = json.loads((output_dir / "topn.json").read_text(encoding="utf-8"))
            self.assertEqual(hotspots[0]["function"], "external_hot")
            self.assertEqual(hotspots[0]["samples"], 7)

    def test_empty_stackcollapse_perf_output_falls_back_to_builtin_parser(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            output_dir = root / "analysis"
            output_dir.mkdir()
            stackcollapse_path = root / "stackcollapse-perf.pl"
            stackcollapse_path.write_text("# fake stackcollapse script\n", encoding="utf-8")
            script_text = """
python  1000 [000] 1.0: cycles:
            7d burn_cpu (/tmp/demo)
            7e worker_loop (/tmp/demo)
            7f main (/tmp/demo)
"""
            stackcollapse_completed = subprocess.CompletedProcess(
                args=[str(stackcollapse_path)],
                returncode=0,
                stdout="",
                stderr="",
            )

            previous = os.environ.get("MINIDROP_STACKCOLLAPSE_PERF")
            os.environ["MINIDROP_STACKCOLLAPSE_PERF"] = str(stackcollapse_path)
            try:
                with mock.patch("subprocess.run", return_value=stackcollapse_completed):
                    collapsed = main.collapse_perf_script(script_text, output_dir)
            finally:
                if previous is None:
                    os.environ.pop("MINIDROP_STACKCOLLAPSE_PERF", None)
                else:
                    os.environ["MINIDROP_STACKCOLLAPSE_PERF"] = previous

            self.assertEqual(collapsed[("main", "worker_loop", "burn_cpu")], 1)

    def test_ebpf_syscall_artifact_generates_distribution(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_path = root / "ebpf.syscalls.txt"
            output_dir = root / "analysis"
            raw_path.write_text(
                """
# Mini-Drop ebpf-syscall raw artifact
@[tracepoint:syscalls:sys_enter_read]: 14
@[tracepoint:syscalls:sys_enter_write]: 8
@[tracepoint:syscalls:sys_enter_epoll_wait]: 3
@[tracepoint:syscalls:sys_enter_read]: 2
""",
                encoding="utf-8",
            )

            stdout = io.StringIO()
            with redirect_stdout(stdout):
                code = main.main_with_argv(
                    [
                        "--task-id",
                        "tsk_ebpf",
                        "--raw-path",
                        str(raw_path),
                        "--output-dir",
                        str(output_dir),
                        "--target-pid",
                        "123",
                        "--sample-rate",
                        "1",
                        "--sample-duration",
                        "5",
                    ]
                )

            self.assertEqual(code, 0)
            hotspots = json.loads((output_dir / "topn.json").read_text(encoding="utf-8"))
            self.assertEqual(hotspots[0]["function"], "read")
            self.assertEqual(hotspots[0]["samples"], 16)
            self.assertTrue((output_dir / "collapsed.txt").exists())
            svg = (output_dir / "flamegraph.svg").read_text(encoding="utf-8")
            self.assertIn("eBPF Syscall Distribution", svg)

    def test_pyspy_raw_artifact_generates_python_profile(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            raw_path = root / "pyspy.raw.txt"
            output_dir = root / "analysis"
            raw_path.write_text(
                """
root;python.main;worker.loop 12
root;python.main;time.sleep 5
root;python.main;worker.loop 3
""",
                encoding="utf-8",
            )

            stdout = io.StringIO()
            with redirect_stdout(stdout):
                code = main.main_with_argv(
                    [
                        "--task-id",
                        "tsk_pyspy",
                        "--raw-path",
                        str(raw_path),
                        "--output-dir",
                        str(output_dir),
                        "--target-pid",
                        "123",
                        "--sample-rate",
                        "99",
                        "--sample-duration",
                        "5",
                    ]
                )

            self.assertEqual(code, 0)
            hotspots = json.loads((output_dir / "topn.json").read_text(encoding="utf-8"))
            self.assertEqual(hotspots[0]["function"], "worker.loop")
            self.assertEqual(hotspots[0]["samples"], 15)
            svg = (output_dir / "flamegraph.svg").read_text(encoding="utf-8")
            self.assertIn("py-spy Flamegraph", svg)

    def test_optional_flamegraph_script_is_used(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            script_path = root / "flamegraph.pl"
            script_path.write_text("# fake flamegraph script\n", encoding="utf-8")
            completed = subprocess.CompletedProcess(
                args=[str(script_path)],
                returncode=0,
                stdout="<svg><text>external flamegraph</text></svg>",
                stderr="",
            )

            previous = os.environ.get("MINIDROP_FLAMEGRAPH_PL")
            os.environ["MINIDROP_FLAMEGRAPH_PL"] = str(script_path)
            try:
                output_path = root / "flamegraph.svg"
                with mock.patch("subprocess.run", return_value=completed) as run_mock:
                    main.render_flamegraph(
                        flamegraph_path=output_path,
                        task_id="tsk_external",
                        target_pid=123,
                        sample_rate=99,
                        sample_duration=15,
                        hotspots=[{"function": "hot.loop", "samples": 10, "percent": 100.0}],
                        collapsed={("root", "hot.loop"): 10},
                        collapsed_text="root;hot.loop 10\n",
                        title="External Flamegraph",
                        subtitle="test",
                    )
                    run_mock.assert_called_once()
            finally:
                if previous is None:
                    os.environ.pop("MINIDROP_FLAMEGRAPH_PL", None)
                else:
                    os.environ["MINIDROP_FLAMEGRAPH_PL"] = previous

            svg = output_path.read_text(encoding="utf-8")
            self.assertIn("external flamegraph", svg)

    def test_missing_flamegraph_script_falls_back_to_builtin_svg(self) -> None:
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp)
            previous = os.environ.get("MINIDROP_FLAMEGRAPH_PL")
            os.environ["MINIDROP_FLAMEGRAPH_PL"] = str(root / "missing-flamegraph.pl")
            try:
                output_path = root / "flamegraph.svg"
                main.render_flamegraph(
                    flamegraph_path=output_path,
                    task_id="tsk_builtin",
                    target_pid=123,
                    sample_rate=99,
                    sample_duration=15,
                    hotspots=[{"function": "hot.loop", "samples": 10, "percent": 100.0}],
                    collapsed={("root", "hot.loop"): 10},
                    collapsed_text="root;hot.loop 10\n",
                    title="Builtin Flamegraph",
                    subtitle="test",
                )
            finally:
                if previous is None:
                    os.environ.pop("MINIDROP_FLAMEGRAPH_PL", None)
                else:
                    os.environ["MINIDROP_FLAMEGRAPH_PL"] = previous

            svg = output_path.read_text(encoding="utf-8")
            self.assertIn("Builtin Flamegraph", svg)
            self.assertIn("rect", svg)


if __name__ == "__main__":
    raise SystemExit(unittest.main())
