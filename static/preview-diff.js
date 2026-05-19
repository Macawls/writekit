(function () {
    var token = window.__previewToken;
    if (!token) return;

    var basePath = window.location.pathname.replace(/\/+$/, '');
    var currentVersion = window.__previewVersion || 0;
    var latestVersion = window.__previewLatest || currentVersion;
    var showDiff = window.__previewShowDiff !== false;

    var loader = document.getElementById('preview-loader');
    var prevBtn = document.getElementById('preview-prev');
    var nextBtn = document.getElementById('preview-next');
    var versionLabel = document.getElementById('preview-version-label');
    var diffToggle = document.getElementById('preview-show-diff');
    var content = document.querySelector('.post-content');

    function setLoader(active) {
        if (!loader) return;
        loader.classList.toggle('active', !!active);
    }

    function setDisabled(el, disabled) {
        if (!el) return;
        if (disabled) {
            el.setAttribute('aria-disabled', 'true');
            el.setAttribute('data-disabled', 'true');
        } else {
            el.removeAttribute('aria-disabled');
            el.removeAttribute('data-disabled');
        }
    }

    function setCookie(name, value) {
        var maxAge = 60 * 60 * 24 * 365;
        document.cookie = name + '=' + value + '; path=/; max-age=' + maxAge + '; samesite=lax';
    }

    function updateChrome() {
        if (versionLabel) {
            versionLabel.innerHTML = String(currentVersion) +
                '<span class="slash">/</span><span class="denom">' + String(latestVersion) + '</span>';
        }
        setDisabled(prevBtn, currentVersion <= 1);
        setDisabled(nextBtn, currentVersion >= latestVersion);
        if (nextBtn) {
            if (currentVersion < latestVersion) nextBtn.classList.add('has-new');
            else nextBtn.classList.remove('has-new');
        }
        if (prevBtn) prevBtn.setAttribute('href', '?v=' + Math.max(1, currentVersion - 1));
        if (nextBtn) nextBtn.setAttribute('href', '?v=' + Math.min(latestVersion, currentVersion + 1));
    }

    function makeBlock(anchor, html, cls) {
        var d = document.createElement('div');
        d.className = 'diff-block' + (cls ? ' ' + cls : '');
        if (anchor) d.setAttribute('data-block-anchor', anchor);
        d.innerHTML = html;
        return d;
    }

    function renderHunks(hunks) {
        if (!content) return;
        var frag = document.createDocumentFragment();
        for (var i = 0; i < hunks.length; i++) {
            var h = hunks[i];
            if (h.op === 'keep') {
                frag.appendChild(makeBlock(h.anchor, h.html, ''));
            } else if (h.op === 'add') {
                frag.appendChild(makeBlock(h.anchor, h.html, showDiff ? 'diff-static-add' : ''));
            } else if (h.op === 'del') {
                if (showDiff) {
                    frag.appendChild(makeBlock(h.anchor, h.oldHtml || '', 'diff-static-del'));
                }
            } else if (h.op === 'replace') {
                if (showDiff) {
                    frag.appendChild(makeBlock(null, h.oldHtml || '', 'diff-static-replace-old'));
                    frag.appendChild(makeBlock(h.anchor, h.html, 'diff-static-replace-new'));
                } else {
                    frag.appendChild(makeBlock(h.anchor, h.html, ''));
                }
            }
        }
        content.innerHTML = '';
        content.appendChild(frag);
    }

    function renderFullHTML(html) {
        if (!content) return;
        content.innerHTML = html;
    }

    function renderVersion(target) {
        if (target < 1) {
            currentVersion = 0;
            if (content) content.innerHTML = '';
            updateChrome();
            try { window.history.replaceState(null, '', basePath); } catch (_) {}
            return Promise.resolve();
        }
        var from = target - 1;
        var url = basePath + '/diff?to=' + target + '&from=' + from;
        setLoader(true);
        return fetch(url, { credentials: 'same-origin' })
            .then(function (r) { return r.ok ? r.json() : null; })
            .then(function (data) {
                setLoader(false);
                if (!data) return;
                currentVersion = data.to;
                if (currentVersion > latestVersion) latestVersion = currentVersion;
                if (!showDiff && data.fullHTML) {
                    renderFullHTML(data.fullHTML);
                } else if (data.hunks) {
                    renderHunks(data.hunks);
                }
                updateChrome();
                try {
                    var search = '?v=' + currentVersion;
                    if (window.location.search !== search) {
                        window.history.replaceState(null, '', basePath + search);
                    }
                } catch (_) {}
            })
            .catch(function () { setLoader(false); });
    }

    if (prevBtn) {
        prevBtn.addEventListener('click', function (e) {
            e.preventDefault();
            if (prevBtn.getAttribute('data-disabled') === 'true') return;
            renderVersion(currentVersion - 1);
        });
    }
    if (nextBtn) {
        nextBtn.addEventListener('click', function (e) {
            e.preventDefault();
            if (nextBtn.getAttribute('data-disabled') === 'true') return;
            renderVersion(currentVersion + 1);
        });
    }

    if (diffToggle) {
        diffToggle.addEventListener('change', function () {
            showDiff = diffToggle.checked;
            setCookie('wk_preview_diff', showDiff ? 'on' : 'off');
            if (currentVersion >= 1) renderVersion(currentVersion);
        });
    }

    updateChrome();

    var activityRoot = (function () {
        var n = document.createElement('div');
        n.className = 'preview-activity';
        document.body.appendChild(n);
        return n;
    })();
    var activeToast = null;

    function clearActive() {
        if (activeToast && activeToast.parentNode) {
            activeToast.parentNode.removeChild(activeToast);
        }
        activeToast = null;
    }

    function spawnToast(opts) {
        clearActive();
        var t = document.createElement('div');
        t.className = 'preview-toast';
        var iconSvg = opts.ok
            ? '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"><polyline points="20 6 9 17 4 12"/></svg>'
            : '<svg width="11" height="11" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 20h9"/><path d="M16.5 3.5a2.121 2.121 0 113 3L7 19l-4 1 1-4z"/></svg>';
        t.innerHTML = '<span class="pt-icon">' + iconSvg + '</span>' +
            '<span class="pt-title">' + opts.title + '</span>' +
            (opts.meta ? '<span class="pt-meta">' + opts.meta + '</span>' : '') +
            (opts.dots ? '<span class="pt-dots"><span></span><span></span><span></span></span>' : '');
        activityRoot.appendChild(t);
        activeToast = t;
        if (opts.fadeAfter) {
            setTimeout(function () {
                if (t.parentNode) {
                    t.classList.add('fade');
                    setTimeout(function () { if (t.parentNode) t.parentNode.removeChild(t); }, 400);
                }
                if (activeToast === t) activeToast = null;
            }, opts.fadeAfter);
        }
    }

    var es = new EventSource(basePath + '/events');
    es.onmessage = function (e) {
        var msg = e.data;
        if (msg === 'saving') {
            setLoader(true);
            spawnToast({
                title: (window.__previewClient || 'AI client') + ' is writing',
                dots: true,
                ok: false
            });
            return;
        }
        if (msg.indexOf('rendered') === 0) {
            setLoader(false);
            var parts = msg.split(':');
            var ver = parts.length > 1 ? parseInt(parts[1], 10) : latestVersion + 1;
            if (isNaN(ver)) return;
            var oldLatest = latestVersion;
            if (ver > latestVersion) latestVersion = ver;
            spawnToast({
                title: 'Saved',
                meta: 'v' + ver,
                ok: true,
                fadeAfter: 3200
            });
            if (currentVersion === oldLatest) {
                window.location.assign(basePath + '?v=' + ver);
            } else {
                updateChrome();
            }
        }
    };
    es.onerror = function () {
        setTimeout(function () { es.close(); }, 30000);
    };
})();
