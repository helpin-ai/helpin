#!/usr/bin/env python3
"""Drive the capture endpoint at a fixed arrival rate and report latency."""

import argparse
import asyncio
import json
import math
import time
import uuid
from collections import Counter
from pathlib import Path

import httpx


def percentile(values: list[float], fraction: float) -> float:
    if not values:
        return 0.0
    ordered = sorted(values)
    index = min(len(ordered) - 1, max(0, math.ceil(fraction * len(ordered)) - 1))
    return ordered[index]


async def run(args: argparse.Namespace) -> dict:
    total = args.rate * args.duration
    total_requests = math.ceil(total / args.batch_size)
    run_id = args.run_id or f"sustained-{uuid.uuid4().hex[:12]}"
    # An unbounded queue is intentional: the scheduler remains open-loop even
    # when the service falls behind, and schedule lag exposes the backlog.
    queue: asyncio.Queue[tuple[int, float] | None] = asyncio.Queue()
    latencies_ms: list[float] = []
    schedule_lag_ms: list[float] = []
    statuses: Counter[str] = Counter()
    successful_events = 0
    started = time.monotonic()

    timeout = httpx.Timeout(args.timeout)
    limits = httpx.Limits(
        max_connections=args.workers,
        max_keepalive_connections=args.workers,
    )

    async with httpx.AsyncClient(timeout=timeout, limits=limits) as client:
        async def worker() -> None:
            nonlocal successful_events
            while True:
                item = await queue.get()
                if item is None:
                    queue.task_done()
                    return
                request_sequence, scheduled_at = item
                request_started = time.monotonic()
                schedule_lag_ms.append((request_started - scheduled_at) * 1000)
                first_event = request_sequence * args.batch_size
                event_count = min(args.batch_size, total - first_event)
                events = []
                for offset in range(event_count):
                    sequence = first_event + offset
                    visitor = sequence % args.visitors
                    events.append({
                        "api_key": args.token,
                        "event_id": f"{run_id}-{sequence}",
                        "event_type": "sustained_test_event",
                        "url": "https://example.test/sustained",
                        "user": {
                            "anonymous_id": f"{run_id}-visitor-{visitor}",
                            "id": f"{run_id}-user-{visitor}",
                        },
                        "event_attributes": {
                            "load_test_run": run_id,
                            "sequence": sequence,
                        },
                        "src": "sustained-e2e",
                    })
                payload = events[0] if args.batch_size == 1 else events
                try:
                    response = await client.post(
                        args.url,
                        headers={
                            "content-type": "application/json",
                            "x-auth-token": args.token,
                            "user-agent": "HelpinSustainedTest/1.0",
                            "x-forwarded-for": "203.0.113.42",
                        },
                        json=payload,
                    )
                    statuses[str(response.status_code)] += 1
                    if response.status_code in (200, 202):
                        successful_events += event_count
                except Exception as error:  # the error type is part of the report
                    statuses[f"error:{type(error).__name__}"] += 1
                finally:
                    latencies_ms.append((time.monotonic() - request_started) * 1000)
                    queue.task_done()

        workers = [asyncio.create_task(worker()) for _ in range(args.workers)]
        interval = args.batch_size / args.rate
        progress_interval = max(1, math.ceil(args.rate * 30 / args.batch_size))
        for request_sequence in range(total_requests):
            scheduled_at = started + request_sequence * interval
            delay = scheduled_at - time.monotonic()
            if delay > 0:
                await asyncio.sleep(delay)
            await queue.put((request_sequence, scheduled_at))
            if request_sequence and request_sequence % progress_interval == 0:
                elapsed = time.monotonic() - started
                print(
                    f"scheduled_events={min(request_sequence * args.batch_size, total)}/{total} "
                    f"elapsed={elapsed:.1f}s "
                    f"queue={queue.qsize()} responses={sum(statuses.values())}",
                    flush=True,
                )

        await queue.join()
        for _ in workers:
            await queue.put(None)
        await asyncio.gather(*workers)

    elapsed = time.monotonic() - started
    completed_rate = total / elapsed
    maintained_target_rate = completed_rate >= args.rate * 0.99
    return {
        "run_id": run_id,
        "target_rate_per_second": args.rate,
        "target_duration_seconds": args.duration,
        "target_requests": total,
        "target_http_requests": total_requests,
        "batch_size": args.batch_size,
        "visitors": args.visitors,
        "workers": args.workers,
        "wall_duration_seconds": round(elapsed, 3),
        "completed_event_rate_per_second": round(completed_rate, 3),
        "maintained_target_rate": maintained_target_rate,
        "successful_events": successful_events,
        "successful_http_requests": statuses.get("200", 0) + statuses.get("202", 0),
        "statuses": dict(statuses),
        "latency_ms": {
            "p50": round(percentile(latencies_ms, 0.50), 3),
            "p95": round(percentile(latencies_ms, 0.95), 3),
            "p99": round(percentile(latencies_ms, 0.99), 3),
            "max": round(max(latencies_ms, default=0.0), 3),
        },
        "schedule_lag_ms": {
            "p50": round(percentile(schedule_lag_ms, 0.50), 3),
            "p95": round(percentile(schedule_lag_ms, 0.95), 3),
            "p99": round(percentile(schedule_lag_ms, 0.99), 3),
            "max": round(max(schedule_lag_ms, default=0.0), 3),
        },
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--url", default="http://127.0.0.1:3000/api/v1/event")
    parser.add_argument("--token", default="e2e-server-secret")
    parser.add_argument("--rate", type=int, default=300)
    parser.add_argument("--duration", type=int, default=900)
    parser.add_argument("--workers", type=int, default=64)
    parser.add_argument("--batch-size", type=int, default=1)
    parser.add_argument("--visitors", type=int, default=10_000)
    parser.add_argument("--timeout", type=float, default=2.0)
    parser.add_argument("--run-id")
    parser.add_argument("--output", type=Path)
    args = parser.parse_args()
    if min(args.rate, args.duration, args.workers, args.visitors, args.batch_size) <= 0:
        parser.error("rate, duration, workers, visitors, and batch-size must be positive")
    return args


def main() -> None:
    args = parse_args()
    result = asyncio.run(run(args))
    rendered = json.dumps(result, indent=2, sort_keys=True)
    print(rendered)
    if args.output:
        args.output.write_text(rendered + "\n")
    if (
        result["successful_events"] != result["target_requests"]
        or not result["maintained_target_rate"]
    ):
        raise SystemExit(1)


if __name__ == "__main__":
    main()
