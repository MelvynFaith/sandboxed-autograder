"""API-side adversarial checks of SRS 11.3, run against a live backend.

  ADMIN_EMAIL=... ADMIN_PASSWORD=... python3 infra/qa/adversarial_api.py [--base http://127.0.0.1:8080]

Needs a running backend and its administrator account (from the environment, or from --env FILE with
ADMIN_EMAIL=... and ADMIN_PASSWORD=... lines). Never run it against a database with real data: it
creates users, Courses and 200 rejected uploads. Makes two lecturers with a Course each,
three students (two in Course A, one in Course B only) and checks, with the expectation of the SRS
row in brackets: IDOR [Akses objek milik pengguna lain], cross-class access [Akses lintas kelas],
role escalation [Eskalasi peran], session after logout / password change [Sesi setelah logout],
hidden test case leaks [Kebocoran test case tersembunyi], self-registration role [FR-19] and the
submission flood [Banjir submission]. Stdlib only; passwords are random and stay in memory.
"""
import argparse
import json
import os
import pathlib
import secrets
import string
import sys
import time
import urllib.error
import urllib.request
import uuid

HERE = pathlib.Path(__file__).parent
ALPHABET = string.ascii_letters + string.digits
LIMITS = {"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 8, "timeout_s": 2,
          "max_output_bytes": 65536, "tmpfs_size_mb": 16}
GOOD = b"print(int(input()) * 2)\n"
results = []


def call(base, method, path, token=None, body=None, raw=None, content_type=None):
    headers, data = {}, raw
    if body is not None:
        data = json.dumps(body).encode()
        headers["Content-Type"] = "application/json"
    if content_type:
        headers["Content-Type"] = content_type
    if token:
        headers["Authorization"] = "Bearer " + token
    request = urllib.request.Request(base + path, data=data, method=method, headers=headers)
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            text = response.read().decode()
            return response.status, (json.loads(text) if text and "json" in response.headers.get("Content-Type", "") else text)
    except urllib.error.HTTPError as error:
        text = error.read().decode()
        try:
            return error.code, json.loads(text)
        except ValueError:
            return error.code, text


def multipart(assignment_id, filename, content):
    boundary = uuid.uuid4().hex
    parts = [
        f'--{boundary}\r\nContent-Disposition: form-data; name="assignment_id"\r\n\r\n{assignment_id}\r\n'.encode(),
        (f'--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="{filename}"\r\n'
         "Content-Type: text/x-python\r\n\r\n").encode() + content + b"\r\n",
        f"--{boundary}--\r\n".encode(),
    ]
    return b"".join(parts), f"multipart/form-data; boundary={boundary}"


def read_env(path):
    values = {}
    for line in pathlib.Path(path).read_text().splitlines():
        if "=" in line and not line.startswith("#"):
            key, value = line.split("=", 1)
            values[key] = value.strip("'")
    return values


def admin_env(env_file):
    """ADMIN_EMAIL and ADMIN_PASSWORD, from env_file if given, else from the environment."""
    if env_file:
        return read_env(env_file)
    return {k: os.environ[k] for k in ("ADMIN_EMAIL", "ADMIN_PASSWORD")}


def check(name, ok, detail):
    results.append(ok)
    print(f"{'ok  ' if ok else 'FAIL'}  {name}: {detail}")


def need(status, payload, want, what):
    if status != want:
        sys.exit(f"SETUP FAILED {what}: HTTP {status} {payload}")
    return payload


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--base", default="http://127.0.0.1:8080")
    parser.add_argument("--env", help="file with ADMIN_EMAIL and ADMIN_PASSWORD lines; default: the environment")
    args = parser.parse_args()
    base, env, run = args.base, admin_env(args.env), secrets.token_hex(3)
    secret = lambda: "".join(secrets.choice(ALPHABET) for _ in range(20))
    api = lambda *a, **k: call(base, *a, **k)

    status, payload = api("POST", "/api/v1/auth/login", body={"email": env["ADMIN_EMAIL"], "password": env["ADMIN_PASSWORD"]})
    admin = need(status, payload, 200, "admin login")["access_token"]

    users = {}  # key -> dict(id, email, password, token, refresh)

    def make(key, role):
        email, password = f"adv-{key}-{run}@autograder.test", secret()
        status, payload = api("POST", "/api/v1/auth/register", body={"name": f"Adv {key}", "email": email, "password": password})
        user = {"id": need(status, payload, 201, f"register {key}")["id"], "email": email, "password": password}
        if role != "STUDENT":
            need(*api("PUT", f"/api/v1/admin/users/{user['id']}/role", admin, {"role": role}), 200, f"promote {key}")
        status, payload = api("POST", "/api/v1/auth/login", body={"email": email, "password": user["password"]})
        need(status, payload, 200, f"login {key}")
        user["token"], user["refresh"] = payload["access_token"], payload["refresh_token"]
        users[key] = user
        return user

    lec_a, lec_b = make("lecA", "LECTURER"), make("lecB", "LECTURER")
    s1, s2, s3, s4 = (make(k, "STUDENT") for k in ("s1", "s2", "s3", "s4"))

    courses = {}
    for key, lec in (("A", lec_a), ("B", lec_b)):
        status, payload = api("POST", "/api/v1/courses", lec["token"], {"name": f"Adv {key} {run}", "code": f"ADV{key}{run.upper()}", "term": "Gasal 2026/2027"})
        courses[key] = need(status, payload, 201, f"course {key}")["id"]
    for course, student in (("A", s1), ("A", s2), ("B", s3), ("A", s4)):
        lec = lec_a if course == "A" else lec_b
        status, payload = api("POST", f"/api/v1/courses/{courses[course]}/enrollments", lec["token"], {"email": student["email"]})
        need(status, payload, 204 if status == 204 else 201, "enrol")

    iso = lambda t: time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime(t))
    assignments = {}
    for key, lec in (("A", lec_a), ("B", lec_b)):
        status, payload = api("POST", f"/api/v1/courses/{courses[key]}/assignments", lec["token"], {
            "title": f"Adv {key}", "description": "x2", "deadline_at": iso(time.time() + 86400),
            "late_until_at": iso(time.time() + 2 * 86400), "resource_limit": LIMITS})
        assignments[key] = need(status, payload, 201, f"assignment {key}")["id"]
    for external_id, given, want, hidden in (("tc_01", "2\n", "4\n", False), ("tc_02", "21\n", "42\n", True)):
        status, payload = api("POST", f"/api/v1/assignments/{assignments['A']}/testcases", lec_a["token"], {
            "external_id": external_id, "input": given, "expected_output": want, "weight": 50,
            "is_hidden": hidden, "match_mode": "EXACT"})
        need(status, payload, 201, f"test case {external_id}")

    def upload(user, assignment, content=GOOD):
        body, content_type = multipart(assignment, "main.py", content)
        return api("POST", "/api/v1/submissions", user["token"], raw=body, content_type=content_type)

    status, payload = upload(s1, assignments["A"])
    sub1 = need(status, payload, 202, "s1 upload")["id"]
    status, payload = upload(s2, assignments["A"])
    sub2 = need(status, payload, 202, "s2 upload")["id"]
    for _ in range(60):
        done = [api("GET", f"/api/v1/submissions/{i}", u["token"])[1].get("status") for i, u in ((sub1, s1), (sub2, s2))]
        if all(s in ("COMPLETED", "FAILED") for s in done):
            break
        time.sleep(1)

    print("== IDOR: s2 (same course) reads s1's submission")
    for path in ("", "/report", "/code"):
        status, _ = api("GET", f"/api/v1/submissions/{sub1}{path}", s2["token"])
        check(f"GET /submissions/{{id}}{path}", status == 404, f"HTTP {status}, want 404")
    status, _ = api("POST", f"/api/v1/submissions/{sub1}/regrade", s2["token"])
    check("student regrade of another's submission", status in (403, 404), f"HTTP {status}, want 403 or 404")
    status, _ = api("DELETE", f"/api/v1/submissions/{sub1}", s2["token"])
    check("student cancel of another's submission", status == 404, f"HTTP {status}, want 404")
    status, _ = api("GET", f"/api/v1/submissions/{sub1}/code", lec_b["token"])
    check("other Course's lecturer reads the code", status == 404, f"HTTP {status}, want 404")
    status, _ = api("GET", f"/api/v1/assignments/{assignments['A']}/results", lec_b["token"])
    check("other Course's lecturer reads the recap", status == 404, f"HTTP {status}, want 404")

    print("== Cross-class: s3 is enrolled in Course B only")
    status, _ = api("GET", f"/api/v1/assignments/{assignments['A']}", s3["token"])
    check("GET /assignments/{id} of Course A", status == 404, f"HTTP {status}, want 404")
    status, _ = upload(s3, assignments["A"])
    check("upload to an assignment of Course A", status == 404, f"HTTP {status}, want 404")
    status, _ = api("GET", f"/api/v1/assignments/{assignments['A']}/testcases", s3["token"])
    check("test cases of Course A", status == 404, f"HTTP {status}, want 404")

    print("== Role escalation")
    status, _ = api("PUT", f"/api/v1/admin/users/{s1['id']}/role", s1["token"], {"role": "ADMIN"})
    check("student promotes self to ADMIN", status in (403, 404), f"HTTP {status}, want 403 or 404")
    status, payload = api("GET", f"/api/v1/users/me", s1["token"])
    check("role unchanged afterwards", payload.get("role") == "STUDENT", f"role {payload.get('role')}")
    status, payload = api("GET", "/api/v1/admin/audit-logs?page=1&page_size=100", admin)
    rows = payload.get("items", []) if isinstance(payload, dict) else []
    mine = [r for r in rows if r.get("user_id") == s1["id"] and "ROLE" in json.dumps(r).upper()]
    check("denied attempt is in the audit log with the user_id", bool(mine), f"{len(mine)} row(s) of user {s1['id']} mentioning ROLE")
    status, _ = api("PUT", f"/api/v1/admin/users/{s1['id']}/role", lec_a["token"], {"role": "LECTURER"})
    check("lecturer changes a role", status in (403, 404), f"HTTP {status}, want 403 or 404")
    status, _ = api("GET", "/api/v1/admin/users", s1["token"])
    check("student lists users", status in (403, 404), f"HTTP {status}, want 403 or 404")
    status, _ = api("GET", "/api/v1/admin/audit-logs", lec_a["token"])
    check("lecturer reads the audit log", status in (403, 404), f"HTTP {status}, want 403 or 404")
    self_email, self_password = f"adv-self-{run}@autograder.test", secret()
    status, payload = api("POST", "/api/v1/auth/register", body={"name": "Self", "email": self_email, "password": self_password, "role": "ADMIN"})
    role = payload.get("role") if isinstance(payload, dict) else None
    login_status, login_payload = api("POST", "/api/v1/auth/login", body={"email": self_email, "password": self_password})
    made_admin = status == 201 and role != "STUDENT"
    check("self-registration with role=ADMIN", not made_admin, f"HTTP {status}, role {role}; the extra field {'created an account' if status == 201 else 'was refused'}")
    status, _ = api("POST", "/api/v1/courses", s1["token"], {"name": "x", "code": "XSTU", "term": "t"})
    check("student creates a Course", status in (403, 404), f"HTTP {status}, want 403 or 404")

    print("== Session after logout / password change")
    status, _ = api("POST", "/api/v1/auth/refresh", body={"refresh_token": s2["refresh"]})
    check("refresh works before logout (control)", status == 200, f"HTTP {status}")
    status, payload = api("POST", "/api/v1/auth/login", body={"email": s2["email"], "password": s2["password"]})
    refresh = payload["refresh_token"]
    api("POST", "/api/v1/auth/logout", s2["token"], {"refresh_token": refresh})
    status, _ = api("POST", "/api/v1/auth/refresh", body={"refresh_token": refresh})
    check("refresh token after logout", status == 401, f"HTTP {status}, want 401")
    status, payload = api("POST", "/api/v1/auth/login", body={"email": s3["email"], "password": s3["password"]})
    refresh = payload["refresh_token"]
    new = secret()
    status, _ = api("PUT", "/api/v1/users/me/password", s3["token"], {"current_password": s3["password"], "new_password": new})
    check("password change accepted", status in (200, 204), f"HTTP {status}")
    status, _ = api("POST", "/api/v1/auth/refresh", body={"refresh_token": refresh})
    check("refresh token after password change", status == 401, f"HTTP {status}, want 401")
    status, _ = api("POST", "/api/v1/auth/refresh", body={"refresh_token": s3["refresh"]})
    check("older refresh token after password change", status == 401, f"HTTP {status}, want 401")

    print("== Hidden test cases")
    status, payload = api("GET", f"/api/v1/assignments/{assignments['A']}/testcases", s1["token"])
    items = payload.get("items", []) if isinstance(payload, dict) else []
    text = json.dumps(payload)
    seen_hidden = [t for t in items if t.get("external_id") == "tc_02" or t.get("is_hidden")]
    leak = [t for t in seen_hidden if t.get("input") not in (None, "") or t.get("expected_output") not in (None, "")]
    check("student test case list", status == 200 and not leak and "42" not in "".join(str(t.get("expected_output", "")) for t in items),
          f"HTTP {status}, {len(items)} of 2 test cases listed, hidden one {'listed without content' if seen_hidden and not leak else 'not listed' if not seen_hidden else 'LEAKED'}")
    status, report = api("GET", f"/api/v1/submissions/{sub1}/report", s1["token"])
    rep = [r for r in report.get("results", []) if r.get("is_hidden")] if isinstance(report, dict) else []
    leak = [r for r in rep if any(r.get(k) not in (None, "") for k in ("actual_output", "error_message", "input", "expected_output"))]
    check("student report", status == 200 and bool(rep) and not leak, f"HTTP {status}, {len(rep)} hidden result(s), {len(leak)} with content")
    status, payload = api("GET", f"/api/v1/assignments/{assignments['A']}/testcases", lec_a["token"])
    shown = [t for t in payload.get("items", []) if t.get("is_hidden") and t.get("expected_output")]
    check("owner lecturer sees hidden test cases (control)", status == 200 and bool(shown), f"HTTP {status}, {len(shown)} hidden with content")

    print("== Flood: s1 posts 200 uploads in a row; s2 must still be served")
    codes = {}
    for _ in range(200):
        status, payload = upload(s1, assignments["A"])
        codes[status] = codes.get(status, 0) + 1
    check("flood by one account", codes.get(429, 0) >= 199 and codes.get(202, 0) <= 1, f"HTTP codes {dict(sorted(codes.items()))}")
    status, payload = upload(s4, assignments["A"])
    queued = payload.get("id") if status == 202 else None
    start, state = time.time(), None
    while queued and time.time() - start < 60:
        state = api("GET", f"/api/v1/submissions/{queued}", s4["token"])[1].get("status")
        if state in ("COMPLETED", "FAILED"):
            break
        time.sleep(0.5)
    check("another student during the flood", status == 202 and state == "COMPLETED", f"upload HTTP {status}, final {state} after {time.time() - start:.1f}s")

    failed = results.count(False)
    print(f"\n{len(results) - failed} of {len(results)} checks passed")
    sys.exit(1 if failed else 0)


if __name__ == "__main__":
    main()
