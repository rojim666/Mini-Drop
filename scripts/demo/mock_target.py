import math
import time


def main() -> None:
    value = 0.0
    while True:
        value = math.sin(value + 0.5)
        time.sleep(0.01)


if __name__ == "__main__":
    main()
