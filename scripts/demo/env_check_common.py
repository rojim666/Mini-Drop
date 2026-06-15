import platform
from pathlib import Path


def is_linux() -> bool:
    return platform.system().lower() == "linux"


def is_wsl() -> bool:
    if not is_linux():
        return False
    release = platform.release().lower()
    if "microsoft" in release or "wsl" in release:
        return True
    version_path = Path("/proc/version")
    try:
        return "microsoft" in version_path.read_text(encoding="utf-8").lower()
    except OSError:
        return False


def runtime_label() -> str:
    label = "WSL2" if is_wsl() else "Linux"
    return f"{label} runtime detected ({platform.release()})"


def print_next_steps(title: str, commands: list[str]) -> None:
    if not commands:
        return
    print(f"Next steps for {title}:")
    for command in commands:
        print(f"  {command}")
