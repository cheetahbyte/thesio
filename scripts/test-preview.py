#!/usr/bin/env python3
"""Opt-in Linux native preview regression and interactive handoff.

Requires an e2e build, xdotool, wmctrl, and Pillow (python -m pip install pillow).
Creates a fresh disposable vault, compares the SAME page with Main or Chapter
open, resizes the real window, and leaves the app open for visual inspection.
Use --no-assert to capture the blurry baseline. No real documents are touched.
"""
import argparse
import base64
import io
import json
import os
from pathlib import Path
import signal
import socket
import subprocess
import tempfile
import time

from PIL import Image, ImageChops, ImageStat


def wait(check, description, timeout=20):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        value = check()
        if value:
            return value
        time.sleep(.04)
    raise AssertionError(description)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary', required=True)
    parser.add_argument('--display', default=':1')
    parser.add_argument('--state', type=Path, default=Path('/tmp/thesio-preview-session.json'))
    parser.add_argument('--output', type=Path, default=Path(tempfile.gettempdir()))
    parser.add_argument('--label', default='final')
    parser.add_argument('--no-assert', action='store_true')
    args = parser.parse_args()
    binary = Path(args.binary).resolve()
    assert binary.is_file()
    # Stop only this runner's previous disposable process. A reused PID or
    # unrelated desktop session must never be killed.
    if args.state.exists():
        old = json.loads(args.state.read_text())
        proc = Path('/proc') / str(old['pid'])
        if proc.exists() and (proc/'cmdline').read_bytes().split(b'\0')[0] == old['binary'].encode():
            assert Path(old['vault']).parent.name.startswith('thesio-preview-gui-')
            assert ('THESIO_HOME='+old['vault']).encode() in (proc/'environ').read_bytes().split(b'\0')
            os.kill(old['pid'], signal.SIGTERM)
            wait(lambda: not proc.exists() or '\nState:\tZ' in (proc/'status').read_text(), 'old fixture did not stop')
    root = Path(tempfile.mkdtemp(prefix='thesio-preview-gui-', dir=args.output))
    vault = root/'vault'
    vault.mkdir()
    (vault/'Main.typ').write_text('''#set page(width: 400pt, height: 460pt, margin: 28pt)
#set text(size: 11pt)
= Preview sharpness

This page belongs to the main document. Open Chapter to compare the same preview across source files.

#text(size: 8pt)[Small type: Clear lines, fine details, and readable punctuation.]

$ integral_0^1 x^2 dif x = 1/3 quad sum_(i=1)^n i = (n(n+1))/2 $

#line(length: 100%, stroke: 0.5pt)

#image("diagram.svg", width: 75%)

#pagebreak()
#include "Chapter.typ"
''')
    (vault/'Chapter.typ').write_text('''= Chapter detail

This page belongs to the chapter. Both pages should stay readable at the current preview size.

#text(size: 8pt)[Fine print: ABCDEFGHIJKLMNOPQRSTUVWXYZ 0123456789.]

$ a^2 + b^2 = c^2 $

#line(length: 100%, stroke: 0.5pt)
''')
    (vault/'diagram.svg').write_text('<svg xmlns="http://www.w3.org/2000/svg" width="300" height="80"><path d="M0 60 L60 20 L120 45 L180 5 L240 40 L300 10" fill="none" stroke="#2463ad" stroke-width="1"/><path d="M0 79H300M1 0V80" stroke="black" stroke-width="0.5"/></svg>')
    address = '/tmp/'+root.name+'.sock'
    env = {**os.environ, 'DISPLAY': args.display}
    with (root/'app.log').open('w') as log:
        app = subprocess.Popen(['setsid', str(binary)], stdin=subprocess.DEVNULL, stdout=log, stderr=subprocess.STDOUT,
            env={**env, 'THESIO_HOME': str(vault), 'MYGO_AUTOMATION': address,
                'XDG_CONFIG_HOME': str(root/'config'), 'XDG_DATA_HOME': str(root/'data'), 'XDG_CACHE_HOME': str(root/'cache')})
    state = dict(pid=app.pid, binary=str(binary), vault=str(vault), socket=address, evidence=str(root), display=args.display, label=args.label)
    args.state.write_text(json.dumps(state, indent=2))
    print(json.dumps(state), flush=True)
    wait(lambda: Path(address).exists(), 'automation socket missing')
    with socket.socket(socket.AF_UNIX) as connection:
        connection.settimeout(20)
        connection.connect(address)
        stream = connection.makefile('rwb')

        def call(method, **params):
            stream.write((json.dumps(dict(id=1, method=method, params=params))+'\n').encode())
            stream.flush()
            response = json.loads(stream.readline())
            assert 'error' not in response, response
            return response['result']

        def tree():
            return call('tree')

        wait(lambda: call('windows')['windows'], 'native window missing')
        wait(lambda: any(n.get('key') == 'doc:Chapter.typ' for n in tree()['nodes']), 'sidebar missing')
        wid = subprocess.check_output(['xdotool', 'search', '--pid', str(app.pid), '--name', 'Thesio'], env=env, text=True).splitlines()[-1]
        subprocess.run(['wmctrl', '-ir', wid, '-b', 'remove,maximized_vert,maximized_horz'], env=env, check=True)
        subprocess.run(['wmctrl', '-ir', wid, '-e', '0,40,30,1000,740'], env=env, check=True)

        def screenshot():
            result = call('capture')
            return Image.open(io.BytesIO(base64.b64decode(result['png']))).convert('RGB')

        def pane():
            return next(n['bounds'] for n in reversed(tree()['nodes']) if n.get('role') == 'scroll')

        def page_crop(image):
            b = pane()
            # Fixture page is 400x460 pt, centered with 24 DIP preview padding.
            # This runner uses the Linux 1x desktop; HiDPI is tested by ui.Tester.
            width = b['width']-48
            return image.crop((round(b['x']+25), round(b['y']+25),
                round(b['x']+23+width), round(min(b['y']+23+width*460/400, image.height-8))))

        def ready():
            # Opening Chapter asynchronously follows its source page. Keep
            # the comparison anchored to the main page, not that later scroll.
            b = pane()
            call('input', events=[dict(type='wheel', x=b['x']+100, y=b['y']+100, dx=0, dy=-3000)])
            call('idle', timeout=5000)
            first = next((n for n in tree()['nodes'] if n.get('label') == 'Page 1'), None)
            if first is not None and 'busy' in first.get('states', []):
                return None
            image = screenshot()
            crop = page_crop(image)
            # White placeholders do not count as a completed preview.
            ink = sum(crop.convert('L').histogram()[:220])
            return image if ink > 1200 else None

        results = []
        for width, wide in [(1000, False), (1176, False), (1280, False), (1137, False), (1280, True)]:
            crops = []
            for doc in ['Main', 'Chapter']:
                call('act', key='doc:'+doc+'.typ', action='press')
                wait(lambda: any(n.get('label') == 'Document' and n.get('value', '').startswith(('= Chapter detail' if doc == 'Chapter' else '#set page')) for n in tree()['nodes']), 'wrong editor')
                wait(ready, 'initial preview missing')
                stable_since = None
                def resized():
                    nonlocal stable_since
                    geometry = call('windows')['windows'][0]['contentBounds']
                    if geometry['width'] != width or pane()['x']+pane()['width'] != width or screenshot().width != width:
                        stable_since = None
                        subprocess.run(['wmctrl', '-ir', wid, '-b', 'remove,maximized_vert,maximized_horz'], env=env, check=True)
                        subprocess.run(['wmctrl', '-ir', wid, '-e', '0,40,30,'+str(width)+',740'], env=env, check=True)
                        return False
                    if stable_since is None:
                        stable_since = time.monotonic()
                    return time.monotonic()-stable_since > .5
                wait(resized, 'window manager refused stable resize')
                if wide:
                    splitter = [n['bounds'] for n in tree()['nodes'] if n.get('role') == 'splitter'][-1]
                    x, y = splitter['x'], splitter['y']+150
                    call('input', events=[dict(type='pointerDown', x=x, y=y), dict(type='pointerMove', x=500, y=y), dict(type='pointerUp', x=500, y=y)])
                b = pane()
                call('input', events=[dict(type='wheel', x=b['x']+100, y=b['y']+100, dx=0, dy=-3000)])
                image = wait(ready, 'preview stayed blank')
                # Poll stable captures, allowing asynchronous raster publication.
                previous = None
                def settled():
                    nonlocal previous
                    now = ready()
                    if now is None:
                        return None
                    crop = page_crop(now)
                    same = previous is not None and ImageChops.difference(previous, crop).getbbox() is None
                    previous = crop
                    return now if same else None
                time.sleep(.2)
                image = wait(settled, 'preview did not settle')
                crop = page_crop(image)
                if crops and not args.no_assert:
                    def matching():
                        nonlocal image, crop
                        image = ready()
                        if image is None:
                            return False
                        crop = page_crop(image)
                        return crop.size == crops[0].size and sum(ImageStat.Stat(ImageChops.difference(crops[0], crop)).mean)/3 < 1
                    wait(matching, 'the same page became blurred after switching source files')
                crops.append(crop)
                assert image.width == width, 'capture did not use requested window size'
                name = args.label+'-'+doc.lower()+'-'+str(width)+('-wide' if wide else '')
                image.save(root/(name+'.png'))
                (root/(name+'.json')).write_text(json.dumps(tree(), indent=2))
            difference = sum(ImageStat.Stat(ImageChops.difference(*crops)).mean)/3
            b = pane()
            results.append(dict(window_width=width, wide=wide, page_display_width=b['width']-48,
                fit_zoom=(b['width']-48)/400, same_page_mean_difference=difference))
            print(json.dumps(results[-1]), flush=True)
        (root/'measurements.json').write_text(json.dumps(results, indent=2))
        # The first page still maps to Main.typ while Chapter is open.
        b = pane()
        call('input', events=[dict(type='click', x=b['x']+90, y=b['y']+90)])
        wait(lambda: any(n.get('label') == 'Document' and n.get('value', '').startswith('#set page') for n in tree()['nodes']), 'click-to-source did not open Main')
        call('act', key='doc:Chapter.typ', action='press')
        b = pane()
        call('input', events=[dict(type='wheel', x=b['x']+100, y=b['y']+100, dx=0, dy=-3000)])
        wait(ready, 'final preview missing')
        print('READY FOR GUI: Chapter open, Main page visible. Compare Main/Chapter in sidebar and resize the window. Evidence: '+str(root), flush=True)


if __name__ == '__main__':
    main()
