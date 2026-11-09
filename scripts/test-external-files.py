#!/usr/bin/env python3
"""Real desktop regression: go build -tags e2e -o /tmp/thesio .

DISPLAY=:1 python3 scripts/test-external-files.py /tmp/thesio --evidence /tmp/evidence
Add --conflict-timing immediate, before-notification, overdue, or clean to gate
native keyboard edits against the real autosave and filesystem subscription.
Requires a real display (or Xvfb), not a mocked MyGo backend. All files are disposable.
"""

import argparse
import base64
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time


def wait_for(check, description):
    deadline = time.monotonic() + 8
    while time.monotonic() < deadline:
        if check():
            return
        time.sleep(0.03)
    raise AssertionError(description)


def run(binary, evidence, timing=None):
    with tempfile.TemporaryDirectory(prefix="thesio-external-") as directory:
        root = Path(directory)
        vault = root / "vault"
        vault.mkdir()
        (vault / "Main.typ").write_text("= Original\n")
        address = root / "automation.sock"
        log = (root / "app.log").open("w+")
        trace = []
        app = subprocess.Popen([str(binary)], env={**os.environ,
            "THESIO_HOME": str(vault), "MYGO_AUTOMATION": str(address)},
            stdout=log, stderr=log)
        try:
            wait_for(address.exists, "automation socket did not start")
            with socket.socket(socket.AF_UNIX) as connection:
                connection.settimeout(15)
                connection.connect(str(address))
                stream = connection.makefile("rwb")

                def call(method, **params):
                    stream.write((json.dumps({"id": 1, "method": method, "params": params}) + "\n").encode())
                    stream.flush()
                    result = json.loads(stream.readline())
                    assert "error" not in result, result
                    return result["result"]

                def nodes():
                    return call("tree")["nodes"]

                def has_file(name):
                    return any(n.get("key") == "doc:" + name for n in nodes())

                def text():
                    return next((n.get("value") for n in nodes() if n.get("label") == "Document"), None)

                def capture(name):
                    if evidence:
                        picture = call("capture")
                        (evidence / (name + ".png")).write_bytes(base64.b64decode(picture["png"]))
                        (evidence / (name + ".json")).write_text(json.dumps(call("tree"), indent=2))

                def type_at_end(value, before):
                    editor = next(n for n in nodes() if n.get("label") == "Document")
                    assert editor.get("value") == before, "unexpected pre-input buffer"
                    bounds = editor["bounds"]
                    call("input", events=[{"type": "click", "x": bounds["x"] + 90, "y": bounds["y"] + 12}])
                    call("act", id=editor["id"], action="focus")
                    call("idle", timeout=1000)
                    wait_for(lambda: call("tree").get("focus") == editor["id"], "native editor focus missing")
                    call("input", events=[{"type": "key", "key": "CmdOrCtrl+End"}])
                    call("idle", timeout=1000)
                    wait_for(lambda: any(n.get("id") == editor["id"] and n.get("selection") == [len(before), len(before)] for n in nodes()), "native append caret missing")
                    call("input", events=[{"type": "text", "text": value}])
                    wait_for(lambda: text() == before + value, "native keyboard edit missing")

                wait_for(lambda: bool(call("windows")["windows"]), "app did not open a window")
                wait_for(lambda: has_file("Main.typ"), "initial file missing")
                if timing:
                    # Observe the buffer AND disk before replacing the file.
                    # A pause beyond 500ms makes the save timer overdue while
                    # real kernel watcher events accumulate, without fake events.
                    started = time.monotonic()

                    def record(event):
                        trace.append({"event": event,
                            "elapsed_ms": round((time.monotonic() - started) * 1000, 3),
                            "wall_time_ns": time.time_ns(), "buffer": text(),
                            "disk": (vault / "Main.typ").read_text()})

                    original = "= Original\n"
                    wait_for(lambda: text() == original, "initial editor missing")
                    type_at_end("LOCAL BEFORE EXTERNAL", original)
                    local = original + "LOCAL BEFORE EXTERNAL"
                    record("local-keyboard-edit-observed")
                    path = vault / "Main.typ"
                    if timing == "clean":
                        wait_for(lambda: path.read_text() == local, "autosave did not finish")
                        record("autosave-confirmed-on-disk")
                    else:
                        if timing != "immediate":
                            call("idle", timeout=250)
                            record("native-frame-settled-with-dirty-disk")
                        os.kill(app.pid, signal.SIGSTOP)
                        _, status = os.waitpid(app.pid, os.WUNTRACED)
                        assert os.WIFSTOPPED(status), "app did not stop at the timing gate"
                    try:
                        before = path.read_text()
                        assert before == (local if timing == "clean" else original), "timing precondition not satisfied"
                        if timing == "overdue":
                            time.sleep(0.7)  # Deliberately cross the real save deadline.
                        elif timing == "before-notification":
                            # Resume just before autosave is due; the real watch
                            # still has its 50ms debounce after the replacement.
                            time.sleep(0.48)
                        external = "= External replacement\n"
                        path.write_text(external)
                        trace.append({"event": "external-replacement-while-" + timing,
                            "elapsed_ms": round((time.monotonic() - started) * 1000, 3),
                            "wall_time_ns": time.time_ns(), "buffer": local,
                            "disk_before": before, "disk_after": path.read_text()})
                    finally:
                        if timing != "clean":
                            os.kill(app.pid, signal.SIGCONT)
                    if timing == "clean":
                        wait_for(lambda: text() == external, "clean editor did not reload")
                        assert not any("File changed outside" in n.get("label", "") for n in nodes())
                    else:
                        wait_for(lambda: any("File changed outside" in n.get("label", "") for n in nodes()), "dirty conflict warning missing")
                        assert text() == local, "truly pending text lost on external replacement"
                    record("external-change-applied")
                    type_at_end(" LOCAL STILL HERE", external if timing == "clean" else local)
                    expected = (external if timing == "clean" else local) + " LOCAL STILL HERE"
                    time.sleep(0.8)  # A later edit schedules another real autosave.
                    record("after-later-typing-and-autosave-deadline")
                    assert text() == expected, "local text disappeared after later typing"
                    assert path.read_text() == (expected if timing == "clean" else external), "autosave violated conflict handling"
                    capture("timing-" + timing)
                    print("PASS: native keyboard conflict timing", timing, json.dumps(trace))
                    return
                (vault / "External.typ").write_text("= Outside\n")
                wait_for(lambda: has_file("External.typ"), "external create missing from sidebar")
                call("act", key="doc:External.typ", action="press")
                wait_for(lambda: text() == "= Outside\n", "new external file did not open")
                (vault / "External.typ").write_text("= Updated\n")
                wait_for(lambda: text() == "= Updated\n", "clean editor did not reload")
                (vault / "swap.tmp").write_text("= Atomic\n")
                (vault / "swap.tmp").replace(vault / "External.typ")
                wait_for(lambda: text() == "= Atomic\n", "atomic save did not reload")
                capture("external-create-edit-atomic")

                (vault / "External.typ").rename(vault / "Renamed.typ")
                wait_for(lambda: has_file("Renamed.typ") and not has_file("External.typ"), "rename missing")
                assert text() == "= Atomic\n", "rename lost open editor"
                (vault / "Renamed.typ").unlink()
                wait_for(lambda: not has_file("Renamed.typ") and text() is None, "deleted clean editor remained")

                (vault / "Folder" / "Nested").mkdir(parents=True)
                (vault / "Folder" / "Nested" / "Child.typ").write_text("= Nested\n")
                wait_for(lambda: has_file("Folder"), "new directory missing")
                call("act", key="doc:Folder", action="press")
                wait_for(lambda: has_file("Folder/Nested"), "nested directory missing")
                call("act", key="doc:Folder/Nested", action="press")
                wait_for(lambda: has_file("Folder/Nested/Child.typ"), "nested file missing")
                (vault / "Folder").rename(vault / "Moved")
                wait_for(lambda: has_file("Moved/Nested/Child.typ") and not has_file("Folder"), "directory rename missing")
                capture("nested-directory-rename")
                (vault / "Moved" / "Nested" / "Child.typ").unlink()
                (vault / "Moved" / "Nested").rmdir()
                (vault / "Moved").rmdir()
                wait_for(lambda: not has_file("Moved"), "directory deletion missing")

                # Included files must refresh the preview even when the editor's
                # own text has not changed. Word counts come from the compilation.
                (vault / "part.typ").write_text("one")
                (vault / "Main.typ").write_text('#include "part.typ"\n')
                call("act", key="doc:Main.typ", action="press")
                wait_for(lambda: text() == '#include "part.typ"\n', "main document did not open")
                wait_for(lambda: any("1 word" in n.get("label", "") for n in nodes()), "include did not compile")
                capture("include-before")
                (vault / "part.typ").write_text("one two three four five six seven")
                wait_for(lambda: any("7 word" in n.get("label", "") for n in nodes()), "external include did not recompile")
                capture("external-include-preview")

                editor = next(n for n in nodes() if n.get("label") == "Document")
                call("act", id=editor["id"], action="setValue", text="= My unsaved work\n")
                # Before the 500ms autosave, create a conflicting external edit.
                (vault / "Main.typ").write_text("= Their external work\n")
                time.sleep(1)
                assert text() == "= My unsaved work\n", "unsaved buffer overwritten"
                assert (vault / "Main.typ").read_text() == "= Their external work\n", "external disk text overwritten"
                assert any("File changed outside" in n.get("label", "") for n in nodes()), "conflict not explained"
                capture("unsaved-conflict")
                (vault / "Main.typ").unlink()
                time.sleep(0.8)
                assert text() == "= My unsaved work\n", "external deletion lost pending text"
                assert not (vault / "Main.typ").exists(), "autosave recreated an externally deleted file"

                for i in range(8):
                    (vault / f"Burst{i}.typ").write_text(f"= Burst {i}\n")
                wait_for(lambda: all(has_file(f"Burst{i}.typ") for i in range(8)), "burst files missing")
                for i in range(8):
                    (vault / f"Burst{i}.typ").unlink()
                wait_for(lambda: all(not has_file(f"Burst{i}.typ") for i in range(8)), "burst deletes missing")
                print("PASS: native GUI external create/edit/atomic save/rename/delete/nested directories/include preview/unsaved conflict/bursts")
        except BaseException:
            log.flush()
            log.seek(0)
            print(log.read())
            raise
        finally:
            app.terminate()
            try:
                app.wait(timeout=5)
            except subprocess.TimeoutExpired:
                app.kill()
                app.wait()
            if evidence:
                log.flush()
                log.seek(0)
                (evidence / ("app-" + (timing or "all") + ".log")).write_text(log.read())
                if timing:
                    (evidence / ("timing-" + timing + "-trace.json")).write_text(json.dumps(trace, indent=2))
            log.close()


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("binary", type=Path)
    parser.add_argument("--evidence", type=Path)
    parser.add_argument("--conflict-timing", choices=["immediate", "before-notification", "overdue", "clean"],
        help="Control whether replacement precedes autosave, outwaits a paused save timer, or follows a confirmed save")
    args = parser.parse_args()
    if args.evidence:
        args.evidence.mkdir(parents=True, exist_ok=True)
    run(args.binary.resolve(), args.evidence, args.conflict_timing)
