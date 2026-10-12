# Serves this directory on 127.0.0.1:8765 so the dev page can fetch the fake scripts (screenshot-fakes.js, composer-fakes.js).
import functools, http.server, pathlib

class Handler(http.server.SimpleHTTPRequestHandler):
    def end_headers(self):
        self.send_header('Access-Control-Allow-Origin', 'http://localhost:34115')
        super().end_headers()

here = pathlib.Path(__file__).resolve().parent
http.server.ThreadingHTTPServer(('127.0.0.1', 8765), functools.partial(Handler, directory=str(here))).serve_forever()
