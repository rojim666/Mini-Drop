import math
import os
import time


def burn_matrix(seed: float) -> float:
    total = seed
    for index in range(900):
        total += math.sqrt((index % 97) + 1.0) * math.sin(total + index)
    return total


def burn_hash(seed: int) -> int:
    value = seed
    for index in range(1200):
        value = ((value << 5) - value + index) & 0xFFFFFFFF
    return value


def burn_runtime_mix(value: float, seed: int) -> tuple[float, int]:
    value = burn_matrix(value)
    seed = burn_hash(seed)
    return value, seed


def main() -> None:
    print(f"target_pid={os.getpid()}", flush=True)
    value = 0.25
    seed = 17
    while True:
        value, seed = burn_runtime_mix(value, seed)
        if seed % 97 == 0:
            time.sleep(0.001)


if __name__ == "__main__":
    main()
