import { getChromeBgLightness, getChromeBgSaturate, getChromeBgHueRotate, processHtml } from './emailBodyHtml'

// Script injected into the sandboxed iframe (moved verbatim from EmailBody.svelte).
const iframeScript = `
      function sendHeight() {
        var height = document.body.scrollHeight;
        window.parent.postMessage({ type: 'iframe-height', height: height }, '*');
      }

      function attachImageHandlers() {
        document.querySelectorAll('img').forEach(function(img) {
          if (!img.dataset.heightHandlerAttached) {
            img.dataset.heightHandlerAttached = 'true';
            img.onload = sendHeight;
            img.onerror = sendHeight;
          }
        });
      }

      window.addEventListener('message', function(e) {
        if (e.data?.type === 'select-all') {
          var range = document.createRange();
          range.selectNodeContents(document.body);
          var selection = window.getSelection();
          if (selection) {
            selection.removeAllRanges();
            selection.addRange(range);
          }
          return;
        }
        if (e.data?.type === 'inline-images' && e.data.images) {
          var images = e.data.images;
          var replaced = 0;
          Object.keys(images).forEach(function(cid) {
            // querySelectorAll (not querySelector): a body can legitimately
            // reference the same cid: from multiple <img> tags — e.g. the
            // composer emits cid:c1 twice when the user pastes the same
            // image twice and the dedup collapses both into one inline
            // attachment. Single-querySelector would leave every <img>
            // except the first stuck on the loading placeholder.
            var imgs = document.querySelectorAll('img[data-cid="' + cid + '"]');
            imgs.forEach(function(img) {
              img.src = images[cid];
              img.removeAttribute('data-cid');
              replaced++;
            });
          });
          if (replaced > 0) {
            attachImageHandlers();
            setTimeout(sendHeight, 50);
            setTimeout(sendHeight, 150);
            setTimeout(sendHeight, 300);
          }
        }
        if (e.data?.type === 'request-print-html') {
          var clone = document.body.cloneNode(true);
          clone.querySelectorAll('script').forEach(function(s){ s.remove(); });
          window.parent.postMessage({ type: 'print-html', html: clone.innerHTML }, '*');
        }
      });

      window.addEventListener('load', function() {
        attachImageHandlers();
        sendHeight();
        window.parent.postMessage({ type: 'iframe-ready' }, '*');
      });

      window.addEventListener('resize', sendHeight);
      new ResizeObserver(sendHeight).observe(document.body);
      setTimeout(sendHeight, 50);
      setTimeout(sendHeight, 200);

      document.addEventListener('click', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          e.preventDefault();
          window.parent.postMessage({ type: 'open-link', url: link.href }, '*');
        }
      });

      // Handle link hover for tooltip
      document.addEventListener('mouseover', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          var rect = link.getBoundingClientRect();
          window.parent.postMessage({
            type: 'link-hover',
            url: link.href,
            x: rect.left,
            y: rect.bottom
          }, '*');
        }
      });

      document.addEventListener('mouseout', function(e) {
        var link = e.target.closest('a');
        if (link && link.href) {
          window.parent.postMessage({ type: 'link-hover-end' }, '*');
        }
      });

      // Handle right-click context menu — always prevent native menu, show custom one
      document.addEventListener('contextmenu', function(e) {
        e.preventDefault();
        var selection = window.getSelection();
        var selectedText = (selection && selection.toString().trim().length > 0) ? selection.toString() : '';
        var link = e.target.closest('a');
        var linkUrl = (link && link.href) ? link.href : '';

        window.parent.postMessage({
          type: 'contextmenu',
          text: selectedText,
          url: linkUrl,
          x: e.clientX,
          y: e.clientY
        }, '*');
      });

      // Forward keyboard events to parent for global shortcuts (only modifier keys and Escape)
      document.addEventListener('keydown', function(e) {
        // Only forward events that need global handling
        if (e.altKey || e.ctrlKey || e.metaKey || e.key === 'Escape') {
          // For pane navigation, blur inside iframe first
          if (e.altKey && (e.key === 'ArrowLeft' || e.key === 'ArrowRight' || e.key === 'h' || e.key === 'l')) {
            if (document.activeElement) {
              document.activeElement.blur();
            }
            document.body.blur();
            window.blur();
          }
          window.parent.postMessage({
            type: 'iframe-keydown',
            key: e.key,
            code: e.code,
            altKey: e.altKey,
            ctrlKey: e.ctrlKey,
            metaKey: e.metaKey,
            shiftKey: e.shiftKey
          }, '*');
        }
      });

      // Notify parent when iframe receives focus/click (but not for links/buttons)
      function isInteractiveElement(el) {
        if (!el) return false;
        var link = el.closest('a');
        if (link && link.href) return true;
        var button = el.closest('button');
        if (button) return true;
        if (el.tagName === 'INPUT' || el.tagName === 'SELECT' || el.tagName === 'TEXTAREA') return true;
        return false;
      }
      document.addEventListener('click', function(e) {
        if (!isInteractiveElement(e.target)) {
          window.parent.postMessage({ type: 'iframe-focus' }, '*');
        }
      });
      document.addEventListener('focus', function(e) {
        if (!isInteractiveElement(e.target)) {
          window.parent.postMessage({ type: 'iframe-focus' }, '*');
        }
      }, true);
    `

// Builds the srcdoc for the sandboxed body iframe. CSP, styles and script are verbatim.
export function buildIframeContent(html: string, applyDarken: boolean, imagesBlocked: boolean): string {
  const processedHtml = processHtml(html, imagesBlocked)
  const imgSrc = imagesBlocked ? "'self' data:" : '* data:'

  // Double-invert: page-level invert + image-level re-invert keeps photos
  // looking normal while flipping text, backgrounds, and CSS-defined colors.
  // Blocked-image placeholders intentionally skip the re-invert so they end
  // up dark too — they're chrome, not content.
  // color-scheme: dark switches the UA-default iframe viewport bg to dark so
  // the rounded-corner edge has no white sliver bleeding through.
  // Invert amount derived from theme's chrome lightness — pure invert(1)
  // produces stark black against themes whose chrome isn't pure black.
  const invertAmount = applyDarken ? 1 - getChromeBgLightness() : 1
  const saturate = applyDarken ? getChromeBgSaturate() : 1
  const hueRotate = applyDarken ? getChromeBgHueRotate() : 0
  // Image filter compensates so photos see net saturate(1) + hue-rotate(0) —
  // html's saturate(S) hue-rotate(H) composed with image's saturate(1/S)
  // hue-rotate(-H) approximately cancels for non-grayscale image content.
  const imageSaturate = 1 / saturate
  const darkenStyles = applyDarken ? `
    html { filter: invert(${invertAmount}) hue-rotate(180deg) saturate(${saturate}) hue-rotate(${hueRotate}deg); background: #fff; color-scheme: dark; }
    img:not([data-blocked-src]), video, iframe, [data-no-invert] { filter: invert(${invertAmount}) hue-rotate(180deg) saturate(${imageSaturate}) hue-rotate(${-hueRotate}deg); }
` : `
    html { color-scheme: light; }
`


  return `<!DOCTYPE html>
<html>
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="Content-Security-Policy" content="default-src 'self' data:; img-src ${imgSrc}; style-src 'unsafe-inline'; script-src 'unsafe-inline';">
  <sty` + `le>${darkenStyles}
    /* Minimal base styles - avoid overriding email's inline styles */
    * { box-sizing: border-box; }
    html, body {
      margin: 0; padding: 0;
      font-family: system-ui, sans-serif;
      font-size: 15px; line-height: 1.5;
      color: #1a1a0a; background-color: white;
      overflow-x: auto; word-wrap: break-word;
      scrollbar-width: none; /* Firefox */
      -ms-overflow-style: none; /* IE/Edge */
    }
    html::-webkit-scrollbar, body::-webkit-scrollbar { display: none; /* Chrome/Safari/WebKit */ }
    body { padding: 16px; }
    img { max-width: 100%; height: auto; }
    /* Ensure empty paragraphs (blank lines) render with visible height */
    p:empty { min-height: 1em; }
    p:has(> br:only-child) { min-height: 1em; }
    img[data-cid] { min-width: 100px; min-height: 60px; }
    a { color: #2563eb; }
    /* Only apply defaults to elements without inline styles */
    blockquote:not([style]) { margin: 0.5em 0; padding-left: 1em; border-left: 3px solid #e5e7eb; color: #6b7280; }
    pre:not([style]) { background: #f3f4f6; padding: 0.5em; border-radius: 4px; overflow-x: auto; }
    /* Remove table/td defaults that conflict with email layouts */
  </sty` + `le>
</head>
<body>
${processedHtml}
<scr` + `ipt>${iframeScript}</scr` + `ipt>
</body>
</html>`
}
