#!/usr/bin/env python3
"""Plots a soak_test.sh metrics CSV (see soak_test.sh's log_metrics()).

Usage:
    python3 plot_soak_metrics.py <metrics.csv> [output.png]

Only stdlib csv + matplotlib — no pandas — so this runs anywhere
matplotlib is already installed, no extra setup needed.
"""

import csv
import sys
from pathlib import Path

import matplotlib.pyplot as plt


def load_rows(csv_path: str) -> dict[str, list[float]]:
	columns = {
		"sim_day": [],
		"sim_hour": [],
		"time_til_next_tick": [],
		"cache_clean_cycles_total": [],
		"ram_usage_mb": [],
		"heap_usage_mb": [],
		"goroutines": [],
		"open_fds": [],
		"connected_users": [],
		"upload_rate": [],
	}

	skipped = 0
	with open(csv_path, newline="") as f:
		for row in csv.DictReader(f):
			# A transient /metrics scrape failure leaves every metric column
			# empty for that sample (soak_test.sh's extract_metric returns
			# '' rather than a placeholder) — drop the whole row instead of
			# fabricating a value for it.
			if any(row[key] == "" for key in columns):
				skipped += 1
				continue
			for key in columns:
				columns[key].append(float(row[key]))

	if skipped:
		print(f"skipped {skipped} row(s) with missing metrics (scrape failures)")

	return columns


def main() -> None:
	if len(sys.argv) < 2:
		print(f"usage: {sys.argv[0]} <metrics.csv> [output.png]", file=sys.stderr)
		sys.exit(1)

	csv_path = sys.argv[1]
	out_path = sys.argv[2] if len(sys.argv) > 2 else str(Path(csv_path).with_suffix(".png"))

	cols = load_rows(csv_path)

	# Simulated time (in days) as the x-axis, rather than wall-clock
	# timestamp — the daemon's behavior is meant to be read against the
	# diurnal simulation soak_test.sh drives, not real elapsed seconds.
	sim_time = [d + h / 24 for d, h in zip(cols["sim_day"], cols["sim_hour"])]

	fig, axes = plt.subplots(3, 1, figsize=(12, 10), sharex=True)

	# Panel 1 — the two inputs feeding computeTickInterval's formula.
	ax1 = axes[0]
	ax1.plot(sim_time, cols["connected_users"], color="tab:blue", label="connected_users")
	ax1.set_ylabel("connected_users", color="tab:blue")
	ax1.tick_params(axis="y", labelcolor="tab:blue")
	ax1b = ax1.twinx()
	ax1b.plot(sim_time, cols["upload_rate"], color="tab:orange", label="upload_rate")
	ax1b.set_ylabel("upload_rate (req/s, smoothed)", color="tab:orange")
	ax1b.tick_params(axis="y", labelcolor="tab:orange")
	ax1.set_title("Inputs: active users & smoothed upload rate")

	# Panel 2 — the output being verified (issue #309): does the tick
	# interval actually respond to activity, and does it stay within
	# [tick_min, tick_max] rather than blowing past either bound?
	ax2 = axes[1]
	ax2.plot(sim_time, cols["time_til_next_tick"], color="tab:green")
	ax2.set_ylabel("time_til_next_tick (s)")
	ax2.set_title("Output: computeTickInterval's live result")

	# Panel 3 — resource cost + cumulative dump-cycle count, so a longer
	# run's slow trends (leaks, unbounded growth) are visible alongside
	# the tick behavior above.
	ax3 = axes[2]
	ax3.plot(sim_time, cols["ram_usage_mb"], label="ram_usage_mb")
	ax3.plot(sim_time, cols["heap_usage_mb"], label="heap_usage_mb")
	ax3.plot(sim_time, cols["goroutines"], label="goroutines")
	ax3.plot(sim_time, cols["open_fds"], label="open_fds")
	ax3.plot(sim_time, cols["cache_clean_cycles_total"], label="cache_clean_cycles_total", linestyle="--")
	ax3.set_ylabel("count / MB")
	ax3.set_xlabel("simulated day")
	ax3.set_title("Resource usage & cumulative cache-clean cycles")
	ax3.legend(loc="upper left", fontsize=8)

	fig.tight_layout()
	fig.savefig(out_path, dpi=150)
	print(f"wrote {out_path}")


if __name__ == "__main__":
	main()
