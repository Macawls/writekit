(function () {
    var modal = document.getElementById('search-modal');
    var trigger = document.querySelector('.search-trigger');
    var input = document.getElementById('search-input');
    var resultsEl = document.getElementById('search-results');
    var emptyEl = document.getElementById('search-empty');
    var hintEl = document.getElementById('search-hint');
    var metaEl = document.getElementById('search-meta');
    var closeBtn = document.querySelector('.search-modal__close');

    if (!modal || !trigger || !input || !resultsEl) return;

    var debounceId = null;
    var currentReq = 0;

    function escapeHtml(s) {
        return String(s == null ? '' : s).replace(/[&<>"']/g, function (c) {
            return { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c];
        });
    }

    function setQueryParam(q) {
        var url = new URL(window.location.href);
        if (q) url.searchParams.set('search', q);
        else url.searchParams.delete('search');
        window.history.replaceState(null, '', url);
    }

    function formatMs(ms) {
        if (ms == null) return '';
        if (ms < 1) return '<1 ms';
        if (ms < 10) return ms.toFixed(1) + ' ms';
        return Math.round(ms) + ' ms';
    }

    function render(results, durationMs) {
        resultsEl.innerHTML = '';
        if (!results.length) {
            emptyEl.hidden = false;
            if (metaEl) {
                metaEl.hidden = false;
                metaEl.textContent = '0 results · ' + formatMs(durationMs);
            }
            return;
        }
        emptyEl.hidden = true;
        if (metaEl) {
            metaEl.hidden = false;
            metaEl.textContent = results.length + ' result' + (results.length === 1 ? '' : 's') + ' · ' + formatMs(durationMs);
        }
        results.forEach(function (r) {
            var li = document.createElement('li');
            li.className = 'search-modal__result';
            li.innerHTML = '<a href="' + escapeHtml(r.url) + '">' +
                '<span class="search-modal__result-title">' + (r.titleHtml || escapeHtml(r.title || '')) + '</span>' +
                (r.collection ? '<span class="search-modal__result-collection">' + escapeHtml(r.collection) + '</span>' : '') +
                (r.snippetHtml ? '<span class="search-modal__result-excerpt">' + r.snippetHtml + '</span>' : '') +
                '</a>';
            resultsEl.appendChild(li);
        });
    }

    function runSearch(q) {
        if (!q) {
            resultsEl.innerHTML = '';
            emptyEl.hidden = true;
            hintEl.hidden = false;
            if (metaEl) metaEl.hidden = true;
            setQueryParam('');
            return;
        }
        hintEl.hidden = true;
        setQueryParam(q);
        var reqId = ++currentReq;
        var clientStart = performance.now();
        fetch('/search.json?q=' + encodeURIComponent(q), { headers: { Accept: 'application/json' } })
            .then(function (res) { return res.ok ? res.json() : { results: [], durationMs: 0 }; })
            .then(function (data) {
                if (reqId !== currentReq) return;
                var ms = (typeof data.durationMs === 'number') ? data.durationMs : (performance.now() - clientStart);
                render(data.results || [], ms);
            })
            .catch(function () {
                if (reqId !== currentReq) return;
                render([], performance.now() - clientStart);
            });
    }

    function openModal() {
        if (typeof modal.showModal === 'function') {
            if (!modal.open) modal.showModal();
        } else {
            modal.setAttribute('open', '');
        }
        document.body.classList.add('search-modal-open');
        setTimeout(function () { input.focus(); input.select(); }, 0);
    }

    function closeModal() {
        if (typeof modal.close === 'function' && modal.open) modal.close();
        else modal.removeAttribute('open');
        document.body.classList.remove('search-modal-open');
        setQueryParam('');
    }

    trigger.addEventListener('click', function () {
        openModal();
        runSearch(input.value.trim());
    });

    if (closeBtn) closeBtn.addEventListener('click', closeModal);

    modal.addEventListener('click', function (e) {
        if (e.target === modal) closeModal();
    });

    modal.addEventListener('close', function () {
        document.body.classList.remove('search-modal-open');
        setQueryParam('');
    });
    modal.addEventListener('cancel', function () {
        document.body.classList.remove('search-modal-open');
        setQueryParam('');
    });

    input.addEventListener('input', function () {
        var val = input.value.trim();
        if (debounceId) clearTimeout(debounceId);
        debounceId = setTimeout(function () { runSearch(val); }, 120);
    });

    var initial = new URLSearchParams(window.location.search).get('search');
    if (initial) {
        input.value = initial;
        openModal();
        runSearch(initial);
    }
})();
