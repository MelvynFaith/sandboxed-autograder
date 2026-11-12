"""SRS 11.3 "Kebocoran test case via /proc": another process on the server reads /proc/<pid>/cmdline
(and environ) of the Grading Harness while it grades. Run it on Linux, on the same host as the
backend, as the same user (the strongest reader there is short of root):

  ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/cmdline_probe.py [--base http://127.0.0.1:8080]

It creates an assignment whose hidden test case has a unique marker as input and expected output,
submits a program that takes a while, and scans every /proc/*/cmdline and /proc/*/environ readable to
it, many times a second, until the submission is graded. Prints whether a marker was ever seen.
"""
import argparse
import glob
import os
import pathlib
import secrets
import subprocess
import sys
import threading
import time

sys.path.insert(0, str(pathlib.Path(__file__).parent))
from adversarial_api import ALPHABET, LIMITS, admin_env, call, multipart, need  # noqa: E402


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:8080")
    parser.add_argument("--env", help="file with ADMIN_EMAIL and ADMIN_PASSWORD lines; default: the environment")
    args = parser.parse_args()
    base, env, run = args.base, admin_env(args.env), secrets.token_hex(3)
    marker_in, marker_out = f"MARKERIN{run.upper()}", f"MARKEROUT{run.upper()}"
    pw = lambda: "".join(secrets.choice(ALPHABET) for _ in range(20))
    api = lambda *a, **k: call(base, *a, **k)

    admin = need(*api("POST", "/api/v1/auth/login", body={"email": env["ADMIN_EMAIL"], "password": env["ADMIN_PASSWORD"]}), 200, "admin")["access_token"]
    users = {}
    for key, role in (("lec", "LECTURER"), ("stu", "STUDENT")):
        email, password = f"probe-{key}-{run}@autograder.test", pw()
        uid = need(*api("POST", "/api/v1/auth/register", body={"name": key, "email": email, "password": password}), 201, key)["id"]
        if role != "STUDENT":
            need(*api("PUT", f"/api/v1/admin/users/{uid}/role", admin, {"role": role}), 200, "promote")
        users[key] = (email, need(*api("POST", "/api/v1/auth/login", body={"email": email, "password": password}), 200, "login")["access_token"])
    lec, stu = users["lec"][1], users["stu"][1]
    course = need(*api("POST", "/api/v1/courses", lec, {"name": f"Probe {run}", "code": f"PRB{run.upper()}", "term": "t"}), 201, "course")["id"]
    need(*api("POST", f"/api/v1/courses/{course}/enrollments", lec, {"email": users["stu"][0]}), 204, "enrol")
    iso = lambda t: time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))
    assignment = need(*api("POST", f"/api/v1/courses/{course}/assignments", lec, {
        "title": "probe", "description": "x", "deadline_at": iso(time.time() + 86400),
        "late_until_at": iso(time.time() + 2 * 86400), "resource_limit": {**LIMITS, "timeout_s": 6}}), 201, "assignment")["id"]
    for n in range(1, 4):  # several test cases so the Harness stays alive for a few seconds
        need(*api("POST", f"/api/v1/assignments/{assignment}/testcases", lec, {
            "external_id": f"tc_0{n}", "input": marker_in + "\n", "expected_output": marker_out + "\n",
            "weight": 10, "is_hidden": True, "match_mode": "EXACT"}), 201, "test case")
    program = b"import time\ninput()\ntime.sleep(1.5)\nprint('nope')\n"
    body, content_type = multipart(assignment, "main.py", program)
    submission = need(*api("POST", "/api/v1/submissions", stu, raw=body, content_type=content_type), 202, "upload")["id"]

    seen, scans, stop = set(), [0], threading.Event()
    control = f"CONTROL{run.upper()}"  # a process that does carry a marker in its argv: the scan must find it
    control_proc = subprocess.Popen(["python3", "-c", "import time; time.sleep(120)", control])
    needles = (marker_in.encode(), marker_out.encode(), control.encode())

    def scan():
        while not stop.is_set():
            for path in glob.glob("/proc/[0-9]*/cmdline") + glob.glob("/proc/[0-9]*/environ"):
                try:
                    data = pathlib.Path(path).read_bytes()
                except OSError:
                    continue
                for needle in needles:
                    if needle in data:
                        seen.add((path.split("/")[-1], needle.decode()[:9]))
            scans[0] += 1

    thread = threading.Thread(target=scan)
    thread.start()
    state, start = None, time.time()
    while time.time() - start < 90:
        state = api("GET", f"/api/v1/submissions/{submission}", stu)[1].get("status")
        if state in ("COMPLETED", "FAILED"):
            break
        time.sleep(0.2)
    stop.set()
    thread.join()
    control_proc.kill()
    found_control = any(n == control[:9] for _, n in seen)
    leaked = sorted(s for s in seen if s[1] != control[:9])
    print(f"submission {submission} final status {state}; {scans[0]} full scans of /proc/*/cmdline and environ")
    print(f"control process (marker in argv) found by the scan: {found_control}")
    print("test case markers seen:", leaked if leaked else "none")
    sys.exit(1 if leaked or not found_control or state not in ("COMPLETED", "FAILED") else 0)


if __name__ == "__main__":
    main()
