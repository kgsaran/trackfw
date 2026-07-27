import os
import subprocess
import sys


def test_log_reads_from_configured_roadmap_dir(tmp_path):
    (tmp_path / "trackfw.yaml").write_text("roadmap_dir: custom/roadmaps\n", encoding="utf-8")
    log_dir = tmp_path / "custom" / "roadmaps"
    log_dir.mkdir(parents=True)
    (log_dir / ".trackfw-log").write_text(
        "2026-07-27 10:00  RM.md  wip -> done\n",
        encoding="utf-8",
    )

    env = os.environ.copy()
    env["PYTHONPATH"] = os.path.abspath(os.path.join(os.path.dirname(__file__), ".."))
    result = subprocess.run(
        [sys.executable, "-m", "trackfw.cli", "log", "--tail", "1"],
        cwd=tmp_path,
        env=env,
        text=True,
        capture_output=True,
        check=False,
    )

    assert result.returncode == 0, result.stderr or result.stdout
    assert "RM.md" in result.stdout
