import math
import os
import time


def main() -> None:
    print(f"target_pid={os.getpid()}", flush=True)
    value = 0.0
    while True:
        value = math.sin(value + 0.5)
        time.sleep(0.01)


if __name__ == "__main__":
    main()
