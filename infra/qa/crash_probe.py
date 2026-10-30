"""NFR-04 / SRS 11.2 failure injection against a live backend: kill it while a submission is RUNNING,
let it come back, and see what happens to the submission and to the leftovers.

  ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/crash_probe.py \\
      --kill 'sudo systemctl kill -s SIGKILL autograder; sudo pkill -9 -f sandbox-exec' \\
      --cgroup-glob '/sys/fs/cgroup/system.slice/autograder.service/sbx-*'

Steps: make a Course and an assignment with one test case, upload a program that sleeps for 8 s,
wait for RUNNING, run the --kill command, run the optional --start command (leave it out when systemd
restarts the unit by itself), wait for /healthz, and poll the submission until it is COMPLETED or
FAILED. Prints the states, the time it took, and the sbx-* cgroup directories and sandbox processes
left right after the kill and after the recovery. Exit status 1 if the submission did not finish or
anything was left. Run it only on a database without real data; infra/qa/clean.sql removes its users.
Stdlib only.
"""
import argparse
import glob
import secrets
import subprocess
import sys
import time

from adversarial_api import ALPHABET, LIMITS, admin_env, call, multipart, need


def leftovers(cgroup_glob):
    cgroups = [p for p in glob.glob(cgroup_glob, recursive=True)]
    ps = subprocess.run(["ps", "-eo", "pid,args"], capture_output=True, text=True).stdout.splitlines()
    procs = [line.strip() for line in ps if "sandbox-exec" in line or "time.sleep(8)" in line]
    return cgroups, procs


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:8080")
    parser.add_argument("--env", help="file with ADMIN_EMAIL and ADMIN_PASSWORD lines; default: the environment")
    parser.add_argument("--kill", required=True, help="shell command that kills the backend and the sandbox")
    parser.add_argument("--start", help="shell command that starts the backend again (omit if systemd does)")
    parser.add_argument("--cgroup-glob", default="/sys/fs/cgroup/**/sbx-*", help="where leftover sandbox cgroups would be")
    args = parser.parse_args()
    base, env, run = args.base, admin_env(args.env), secrets.token_hex(3)
    pw = lambda: "".join(secrets.choice(ALPHABET) for _ in range(20))
    api = lambda *a, **k: call(base, *a, **k)

    admin = need(*api("POST", "/api/v1/auth/login", body={"email": env["ADMIN_EMAIL"], "password": env["ADMIN_PASSWORD"]}), 200, "admin")["access_token"]
    users = {}
    for key, role in (("lec", "LECTURER"), ("stu", "STUDENT")):
        email, password = f"inj-{key}-{run}@autograder.test", pw()
        uid = need(*api("POST", "/api/v1/auth/register", body={"name": key, "email": email, "password": password}), 201, key)["id"]
        if role != "STUDENT":
            need(*api("PUT", f"/api/v1/admin/users/{uid}/role", admin, {"role": role}), 200, "promote")
        users[key] = (email, password)
    token = lambda key: need(*api("POST", "/api/v1/auth/login", body={"email": users[key][0], "password": users[key][1]}), 200, "login")["access_token"]
    lec, stu = token("lec"), token("stu")
    course = need(*api("POST", "/api/v1/courses", lec, {"name": f"Inj {run}", "code": f"INJ{run.upper()}", "term": "t"}), 201, "course")["id"]
    need(*api("POST", f"/api/v1/courses/{course}/enrollments", lec, {"email": users["stu"][0]}), 204, "enrol")
    iso = lambda t: time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))
    assignment = need(*api("POST", f"/api/v1/courses/{course}/assignments", lec, {
        "title": "inj", "description": "x", "deadline_at": iso(time.time() + 86400),
        "late_until_at": iso(time.time() + 2 * 86400), "resource_limit": {**LIMITS, "timeout_s": 12}}), 201, "assignment")["id"]
    need(*api("POST", f"/api/v1/assignments/{assignment}/testcases", lec, {
        "external_id": "tc_01", "input": "1\n", "expected_output": "1\n", "weight": 10,
        "is_hidden": False, "match_mode": "EXACT"}), 201, "test case")
    body, content_type = multipart(assignment, "main.py", b"import time\ninput()\ntime.sleep(8)\nprint(1)\n")
    sid = need(*api("POST", "/api/v1/submissions", stu, raw=body, content_type=content_type), 202, "upload")["id"]

    t0, state = time.time(), None
    while time.time() - t0 < 30:
        state = api("GET", f"/api/v1/submissions/{sid}", stu)[1].get("status")
        if state == "RUNNING":
            break
        time.sleep(0.2)
    print(f"submission {sid}: {state} after {time.time() - t0:.1f}s")
    if state != "RUNNING":
        sys.exit("never reached RUNNING; nothing was injected")
    time.sleep(1.0)  # let the program start inside the sandbox
    print("killing the backend and the sandbox")
    subprocess.run(args.kill, shell=True)
    time.sleep(1.0)
    cgroups, procs = leftovers(args.cgroup_glob)
    print(f"right after the kill: {len(cgroups)} sbx-* cgroup dir(s), {len(procs)} sandbox process(es)")
    if args.start:
        subprocess.Popen(args.start, shell=True, stdin=subprocess.DEVNULL, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)

    t1 = time.time()
    while time.time() - t1 < 90:
        try:
            if call(base, "GET", "/healthz")[1].get("status") == "ok":
                break
        except OSError:  # connection refused while the backend is still starting
            pass
        time.sleep(0.5)
    print(f"backend answering /healthz again after {time.time() - t1:.1f}s")
    stu, last, t2 = token("stu"), None, time.time()
    while time.time() - t2 < 240:
        state = api("GET", f"/api/v1/submissions/{sid}", stu)[1].get("status")
        if state != last:
            last = state
            print(f"  +{time.time() - t2:5.1f}s status {state}")
        if state in ("COMPLETED", "FAILED"):
            break
        time.sleep(0.5)
    final = api("GET", f"/api/v1/submissions/{sid}", stu)[1]
    print("final:", {k: final.get(k) for k in ("status", "total_score", "max_score")})
    cgroups, procs = leftovers(args.cgroup_glob)
    print(f"after recovery: {len(cgroups)} sbx-* cgroup dir(s), {len(procs)} sandbox process(es)")
    sys.exit(1 if final.get("status") not in ("COMPLETED", "FAILED") or cgroups or procs else 0)


if __name__ == "__main__":
    main()
